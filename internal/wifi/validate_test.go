package wifi

import (
	"context"
	"errors"
	"testing"
)

func TestValidateJoinRequestMatchesJoin(t *testing.T) {
	got, err := ValidateJoinRequest(JoinRequest{SSID: "Test", BSSID: "02:AB:00:00:00:01", Security: SecurityWPA2, Key: "example-only"})
	if err != nil || got.BSSID != "02:ab:00:00:00:01" || got.Key != "example-only" {
		t.Fatal(got, err)
	}
	service := New(context.Background(), &fakeUbus{}, nil)
	for _, req := range []JoinRequest{
		{Security: SecurityOpen},
		{SSID: "Test", BSSID: "wrong", Security: SecurityOpen},
		{SSID: "Test", Security: SecurityWPA2},
		{SSID: "Test", Security: SecurityOpen, Key: "secret"},
		{SSID: "Test", Security: SecurityOther},
		{SSID: "Test", Security: "wep"},
	} {
		_, shared := ValidateJoinRequest(req)
		_, joined := service.Join(context.Background(), req)
		// ValidationError is comparable, so errors.Is also checks the code.
		if shared == nil || !errors.Is(joined, shared) {
			t.Fatalf("%#v: shared %v, join %v", req, shared, joined)
		}
	}
}
