package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"reflect"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/api"
)

// The committed spec must describe exactly what the daemon serves.
func TestOutputEqualsServedSpec(t *testing.T) {
	var out bytes.Buffer
	if err := render(&out); err != nil {
		t.Fatal(err)
	}
	d := specDeps()
	d.Trusted = func(netip.Addr) bool { return true } // the served spec is LAN-only
	h, _ := api.New(d)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/openapi.json", nil)
	req.RemoteAddr, req.Host = "127.0.0.1:1234", "localhost"
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	var got, want any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("printed spec differs from served spec\nprinted: %s\nserved: %s", out.String(), rec.Body.String())
	}
}
