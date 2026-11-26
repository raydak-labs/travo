package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/statshistory"
)

type fakeStatsHistory struct {
	since    time.Time
	points   []statshistory.Point
	resetErr error
	resets   int
}

func (f *fakeStatsHistory) Points(since time.Time) ([]statshistory.Point, statshistory.Persistence) {
	f.since = since
	return f.points, statshistory.PersistenceDegraded
}

func (f *fakeStatsHistory) Reset() error { f.resets++; return f.resetErr }

func TestStatsHistoryContract(t *testing.T) {
	const path = "/api/v1/stats/history"
	unavailable := newEnv(t)
	cookie, csrf := unavailable.login()
	if rec := unavailable.do(req{method: "GET", path: path, cookie: cookie}); rec.Code != http.StatusServiceUnavailable || errCode(t, rec) != "stats_history_unavailable" {
		t.Fatalf("nil store: %d %s", rec.Code, rec.Body)
	}

	clients := 2
	history := &fakeStatsHistory{points: []statshistory.Point{{At: 1, UptimeSeconds: 60, Clients: &clients, Uplinks: []statshistory.UplinkBytes{}}}}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.StatsHistory = history })
	if rec := e.do(req{method: "GET", path: path}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: %d", rec.Code)
	}
	cookie, csrf = e.login()
	rec := e.do(req{method: "GET", path: path + "?hours=6", cookie: cookie})
	var body statsHistoryBody
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &body) != nil {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}
	if body.IntervalSeconds != 600 || body.RetentionHours != 168 || body.Persistence != "degraded" || len(body.Points) != 1 {
		t.Fatalf("body = %+v", body)
	}
	if got := e.clock.Now().Sub(history.since); got != 6*time.Hour {
		t.Fatalf("since = now - %v, want 6h", got)
	}
	for _, bad := range []string{"0", "169", "x"} {
		if rec := e.do(req{method: "GET", path: path + "?hours=" + bad, cookie: cookie}); rec.Code != http.StatusUnprocessableEntity && rec.Code != http.StatusBadRequest {
			t.Fatalf("hours=%s: %d", bad, rec.Code)
		}
	}

	if rec := e.do(req{method: "DELETE", path: path, cookie: cookie, csrf: csrf}); rec.Code != http.StatusOK || history.resets != 1 {
		t.Fatalf("reset: %d %s", rec.Code, rec.Body)
	}
	history.resetErr = errors.New("read-only fs")
	if rec := e.do(req{method: "DELETE", path: path, cookie: cookie, csrf: csrf}); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("failed reset: %d", rec.Code)
	}
}
