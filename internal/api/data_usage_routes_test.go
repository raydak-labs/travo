package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/datausage"
)

type fakeDataUsage struct {
	err    error
	put    datausage.Budget
	resets int
}

func (f *fakeDataUsage) View(context.Context) (datausage.View, error) {
	return datausage.View{Budget: datausage.DefaultBudget(), BudgetState: datausage.StateOff}, f.err
}

func (f *fakeDataUsage) SetBudget(ctx context.Context, b datausage.Budget) (datausage.View, error) {
	f.put = b
	return f.View(ctx)
}

func (f *fakeDataUsage) ResetBudget(ctx context.Context) (datausage.View, error) {
	f.resets++
	return f.View(ctx)
}

func TestDataUsageRoutes(t *testing.T) {
	const path = "/api/v1/network/usage"
	unavailable := newEnv(t)
	cookie, _ := unavailable.login()
	if rec := unavailable.do(req{method: "GET", path: path, cookie: cookie}); rec.Code != http.StatusServiceUnavailable || errCode(t, rec) != "data_usage_unavailable" {
		t.Fatalf("nil service: %d %s", rec.Code, rec.Body)
	}

	f := &fakeDataUsage{}
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.DataUsage = f })
	if rec := e.do(req{method: "GET", path: path}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: %d", rec.Code)
	}
	cookie, csrf := e.login()
	if rec := e.do(req{method: "GET", path: path, cookie: cookie}); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"budget_state":"off"`) {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}
	body := `{"enabled":true,"limit_bytes":10737418240,"warn_percent":80,"start_day":5,"uplinks":["usbtether"]}`
	if rec := e.do(req{method: "PUT", path: path + "/budget", body: body, cookie: cookie, csrf: csrf}); rec.Code != http.StatusOK || f.put.StartDay != 5 || f.put.Uplinks[0] != "usbtether" {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}
	if rec := e.do(req{method: "DELETE", path: path + "/budget", cookie: cookie, csrf: csrf}); rec.Code != http.StatusOK || f.resets != 1 {
		t.Fatalf("reset: %d %s", rec.Code, rec.Body)
	}
	for code, err := range map[string]error{
		"invalid_request":        &datausage.FieldError{Field: "start_day", Reason: "1 to 28"},
		"data_budget_unreadable": datausage.ErrUnreadable,
		"data_usage_unavailable": datausage.ErrUnavailable,
	} {
		f.err = err
		rec := e.do(req{method: "PUT", path: path + "/budget", body: body, cookie: cookie, csrf: csrf})
		if errCode(t, rec) != code {
			t.Errorf("%s: %d %s", code, rec.Code, rec.Body)
		}
	}
}
