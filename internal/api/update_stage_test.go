package api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/updatestage"
)

type fakeStager struct {
	err  error
	last string
}

func (f *fakeStager) Status() updatestage.StageStatus {
	return updatestage.StageStatus{Status: "idle"}
}

func (f *fakeStager) Stage(_ context.Context, version string) (updatestage.StageStatus, error) {
	f.last = version
	return updatestage.StageStatus{Version: &version, Status: "verified"}, f.err
}

func TestUpdateStageAPI(t *testing.T) {
	stager := &fakeStager{}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.UpdateStage = stager })
	read := req{path: stagePath}
	post := req{method: "POST", path: stagePath, body: `{"version":"1.2.4"}`}
	for _, r := range []req{read, post} {
		if got := e.do(r); got.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated: %d %s", got.Code, got.Body)
		}
	}
	cookie, token := e.login()
	read.cookie, post.cookie = cookie, cookie
	if got := e.do(read); got.Code != http.StatusOK || bodyField(t, got, "status") != "idle" {
		t.Fatalf("read: %d %s", got.Code, got.Body)
	}
	if got := e.do(post); got.Code != http.StatusForbidden || stager.last != "" {
		t.Fatalf("CSRF: %d %s", got.Code, got.Body)
	}
	post.csrf = token
	if got := e.do(post); got.Code != http.StatusOK || bodyField(t, got, "status") != "verified" || stager.last != "1.2.4" {
		t.Fatalf("stage: %d %s", got.Code, got.Body)
	}
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{updatestage.ErrBusy, 409, "stage_busy"},
		{updatestage.ErrChanged, 409, "release_changed"},
		{updatestage.ErrUnavailable, 503, "stage_unavailable"},
		{errors.New("private path"), 502, "stage_failed"},
	} {
		stager.err = tc.err
		got := e.do(post)
		if got.Code != tc.status || bodyField(t, got, "code") != tc.code || bodyField(t, got, "error") == "private path" {
			t.Fatalf("error: %d %s", got.Code, got.Body)
		}
	}
}
