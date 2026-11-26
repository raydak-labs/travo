package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/updateapply"
)

type fakeApplier struct {
	state   updateapply.ApplyStatus
	err     error
	started string
}

func (f *fakeApplier) Status() updateapply.ApplyStatus { return f.state }
func (f *fakeApplier) Begin(_ context.Context, version string) (*updateapply.Accepted, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.started = version
	f.state = updateapply.ApplyStatus{ID: "test-id", Version: version, State: "applying"}
	return &updateapply.Accepted{Status: f.state}, nil
}
func (f *fakeApplier) Recover() (updateapply.ApplyStatus, error) { return f.state, f.err }

func TestUpdateApplyAPI(t *testing.T) {
	apply := &fakeApplier{state: updateapply.ApplyStatus{State: "idle"}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.UpdateApply = apply })
	read := req{path: applyPath}
	post := req{method: "POST", path: applyPath, body: `{"version":"1.2.4"}`}
	recover := req{method: "POST", path: "/api/v1/system/update/recover"}
	for _, r := range []req{read, post, recover} {
		if got := e.do(r); got.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated: %d %s", got.Code, got.Body)
		}
	}
	cookie, token := e.login()
	read.cookie, post.cookie, recover.cookie = cookie, cookie, cookie
	if got := e.do(read); got.Code != http.StatusOK || bodyField(t, got, "state") != "idle" {
		t.Fatalf("read: %d %s", got.Code, got.Body)
	}
	for _, r := range []req{post, recover} {
		if got := e.do(r); got.Code != http.StatusForbidden {
			t.Fatalf("csrf: %d %s", got.Code, got.Body)
		}
	}
	post.csrf, recover.csrf = token, token
	if got := e.do(post); got.Code != http.StatusAccepted || bodyField(t, got, "id") != "test-id" || apply.started != "1.2.4" {
		t.Fatalf("apply: %d %s", got.Code, got.Body)
	}
	if got := e.do(recover); got.Code != http.StatusOK || bodyField(t, got, "state") != "applying" {
		t.Fatalf("recover: %d %s", got.Code, got.Body)
	}
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{updateapply.ErrBusy, 409, "update_busy"},
		{updateapply.ErrStage, 409, "stage_required"},
		{updateapply.ErrUnavailable, 503, "update_unavailable"},
	} {
		apply.err = tc.err
		if got := e.do(post); got.Code != tc.status || bodyField(t, got, "code") != tc.code {
			t.Fatalf("apply error: %d %s", got.Code, got.Body)
		}
	}
}
