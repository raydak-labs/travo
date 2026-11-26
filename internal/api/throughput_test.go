package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/throughput"
)

type throughputSnapshot struct {
	rates []throughput.Rate
	ready bool
}

func (s *throughputSnapshot) Latest() ([]throughput.Rate, bool) { return s.rates, s.ready }

func TestThroughputContract(t *testing.T) {
	snapshot := &throughputSnapshot{}
	e := newEnvWithDevices(t, nil, snapshot)
	const path = "/api/v1/dashboard/throughput"
	if rec := e.do(req{method: "GET", path: path}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated: %d %s", rec.Code, rec.Body)
	}
	cookie, _ := e.login()
	request := req{method: "GET", path: path, cookie: cookie}
	if rec := e.do(request); rec.Code != http.StatusServiceUnavailable || errCode(t, rec) != "throughput_unavailable" {
		t.Fatalf("unavailable: %d %s", rec.Code, rec.Body)
	}
	snapshot.ready = true
	snapshot.rates = []throughput.Rate{}
	checkThroughputBody(t, e, request, `[]`)
	zero := 0.0
	snapshot.rates = []throughput.Rate{{Name: "br-lan"}, {Name: "wan", RXBytesPerSecond: &zero, TXBytesPerSecond: &zero}}
	checkThroughputBody(t, e, request, `[{"name":"br-lan","rx_bytes_per_second":null,"tx_bytes_per_second":null},{"name":"wan","rx_bytes_per_second":0,"tx_bytes_per_second":0}]`)
}

func checkThroughputBody(t *testing.T, e *env, request req, want string) {
	t.Helper()
	rec := e.do(request)
	var body struct {
		Interfaces json.RawMessage `json:"interfaces"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &body) != nil || string(body.Interfaces) != want {
		t.Fatalf("got %d %s, want interfaces %s", rec.Code, rec.Body, want)
	}
}
