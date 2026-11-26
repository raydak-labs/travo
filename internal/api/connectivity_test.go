package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/connectivity"
)

type connectivitySnapshot connectivity.Status

func (s *connectivitySnapshot) Status(context.Context) connectivity.Status {
	return connectivity.Status(*s)
}

func TestConnectivityContract(t *testing.T) {
	snapshot := &connectivitySnapshot{Internet: connectivity.Offline}
	e := newEnvWithDashboard(t, nil, nil, snapshot)
	const path = "/api/v1/dashboard/connectivity"
	if got := e.do(req{method: "GET", path: path}); got.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: %d %s", got.Code, got.Body)
	}
	cookie, _ := e.login()
	for _, want := range []string{
		`{"active_interface":null,"carrier":null,"internet":"offline"}`,
		`{"active_interface":"phy0-sta0","carrier":false,"internet":"online","portal_url":null}`,
		`{"active_interface":"phy0-sta0","carrier":true,"internet":"captive","portal_url":"http://portal.example/login?x=1"}`,
	} {
		var st connectivity.Status
		if err := json.Unmarshal([]byte(want), &st); err != nil {
			t.Fatal(err)
		}
		*snapshot = connectivitySnapshot(st)
		got := e.do(req{method: "GET", path: path, cookie: cookie})
		var body map[string]json.RawMessage
		if got.Code != http.StatusOK || json.Unmarshal(got.Body.Bytes(), &body) != nil {
			t.Fatalf("%d %s", got.Code, got.Body)
		}
		var expected map[string]json.RawMessage
		if err := json.Unmarshal([]byte(want), &expected); err != nil {
			t.Fatal(err)
		}
		for key, value := range expected {
			if string(body[key]) != string(value) {
				t.Fatalf("field %s: got %s want %s", key, body[key], value)
			}
		}
	}
}
