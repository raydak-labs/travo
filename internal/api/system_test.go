package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
)

const wantSystemInfo = `{"hostname":"OpenWrt","model":"GL.iNet GL-AXT1800","board_name":"glinet,gl-axt1800",
 "openwrt":"25.12.6","revision":"r33311-981c978eff","target":"qualcommax/ipq60xx","kernel":"6.12.112",
 "uptime_seconds":224,"load":[0.39,0.24,0.1],
 "memory":{"total_bytes":428646400,"available_bytes":149295104},
 "storage":{"total_bytes":81616896,"available_bytes":69513216},
 "app":{"version":"dev","build_time":""}}`

func TestSystemInfo(t *testing.T) {
	e := newEnv(t)
	mock.SeedSystem(e.dev)
	cookie, csrf := e.login()
	rec := e.do(req{method: "GET", path: "/api/v1/system/info", cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	var got, want map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(wantSystemInfo), &want); err != nil {
		t.Fatal(err)
	}
	delete(got, "$schema")
	g, _ := json.Marshal(got)
	w, _ := json.Marshal(want)
	if string(g) != string(w) {
		t.Fatalf("got  %s\nwant %s", g, w)
	}
}

func TestSystemInfoUnavailable(t *testing.T) {
	e := newEnv(t) // ubus replies not scripted
	cookie, csrf := e.login()
	rec := e.do(req{method: "GET", path: "/api/v1/system/info", cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusServiceUnavailable || errCode(t, rec) != "device_unavailable" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	// a malformed reply is also unavailable, never a partial 200
	e.dev.SetUbusReply("system", "board", []byte(`{}`))
	rec = e.do(req{method: "GET", path: "/api/v1/system/info", cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("bad shape: %d %s", rec.Code, rec.Body)
	}
}

func TestSystemInfoRequiresSession(t *testing.T) {
	e := newEnv(t)
	mock.SeedSystem(e.dev)
	rec := e.do(req{method: "GET", path: "/api/v1/system/info"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}
