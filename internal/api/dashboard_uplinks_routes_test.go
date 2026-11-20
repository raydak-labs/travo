package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/uplinkview"
)

type fakeUplinks struct {
	body uplinkview.Uplinks
	err  error
}

func (f fakeUplinks) Uplinks(context.Context) (uplinkview.Uplinks, error) { return f.body, f.err }

func TestDashboardUplinksRoute(t *testing.T) {
	order := []string{"wan"}
	body := uplinkview.Uplinks{Uplinks: []uplinkview.Uplink{{Name: "wan"}}, Failover: &uplinkview.Failover{Order: order}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.Uplinks = fakeUplinks{body: body} })
	cookie, _ := e.login()
	rec := e.do(req{method: http.MethodGet, path: "/api/v1/dashboard/uplinks", cookie: cookie})
	want := `{"uplinks":[{"name":"wan","configured":false,"up":false,"device":null,"ipv4":null,"ssid":null,"kind":null}],"failover":{"enabled":false,"active":false,"order":["wan"]}}`
	if rec.Code != http.StatusOK || compactJSON(t, rec.Body.Bytes()) != compactJSON(t, []byte(want)) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestDashboardUplinksUnavailable(t *testing.T) {
	for _, reader := range []UplinkReader{nil, fakeUplinks{err: uplinkview.ErrUnavailable}, fakeUplinks{err: errors.New("ubus")}} {
		e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.Uplinks = reader })
		cookie, _ := e.login()
		rec := e.do(req{method: http.MethodGet, path: "/api/v1/dashboard/uplinks", cookie: cookie})
		if rec.Code != http.StatusServiceUnavailable || !json.Valid(rec.Body.Bytes()) {
			t.Errorf("%T: %d %s", reader, rec.Code, rec.Body)
		}
	}
}

func compactJSON(t *testing.T, b []byte) string {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	out, _ := json.Marshal(v)
	return string(out)
}
