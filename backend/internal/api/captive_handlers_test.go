package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/services"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// recordingRunner records every command the service under test shells out, so
// a read path can be checked for mutations by what it ASKED the device to do,
// not by whether it happened to return an error.
type recordingRunner struct {
	calls []string
}

func (r *recordingRunner) Run(name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, strings.TrimSpace(name+" "+strings.Join(args, " ")))
	switch {
	case strings.HasPrefix(name, "ip"), name == "uci" && len(args) > 0 && args[0] == "commit":
		return []byte(""), nil
	case name == "/etc/init.d/dnsmasq":
		return nil, nil
	}
	return nil, fmt.Errorf("not modelled: %s %v", name, args)
}

// GET /api/v1/captive/status must not change anything. It used to call
// MaybeAutoRestoreDNS, so every poll of the captive status — the dashboard does
// it on a timer — committed `dhcp` and `network`, rewrote dnsmasq's resolver
// options and restarted dnsmasq. A read that mutates DNS is indistinguishable
// from an outage when it lands mid-resolve.
//
// The bypass is genuinely active here, which is the only state in which the
// restore path had anything to do: pre-fix, this request committed twice,
// restarted dnsmasq and cleared the bypass.
func TestCaptiveStatusGetDoesNotMutateState(t *testing.T) {
	dir := t.TempDir()
	guardFile := filepath.Join(dir, "captive-dns-in-progress")
	// The guard file is the marker for "bypass active" and holds the backup the
	// restore would apply.
	backup := `{"dnsmasq_noresolv":"1","dnsmasq_servers":["127.0.0.1#5353"],` +
		`"dnsmasq_rebind_protection":"0","time":1}`
	if err := os.WriteFile(guardFile, []byte(backup), 0o600); err != nil {
		t.Fatalf("write bypass guard: %v", err)
	}

	runner := &recordingRunner{}
	// 204 = internet reachable, which is exactly what used to trigger the
	// auto-restore from the read path.
	svc := services.NewCaptiveServiceWithGuard(&services.MockHTTPProber{StatusCode: 204},
		uci.NewMockUCI(), runner, guardFile)
	defer svc.Stop()

	app := fiber.New()
	app.Get("/api/v1/captive/status", CaptiveStatusHandler(svc))

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/captive/status", nil))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want 200; the fix must not turn the endpoint into a no-op: %s",
			resp.StatusCode, body)
	}
	var status struct {
		DNSBypassed      bool `json:"dns_bypassed"`
		CanReachInternet bool `json:"can_reach_internet"`
	}
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	if !status.CanReachInternet || !status.DNSBypassed {
		t.Errorf("status = %s; the read must still report the portal state", body)
	}

	for _, call := range runner.calls {
		if strings.HasPrefix(call, "uci commit") {
			t.Errorf("GET /captive/status committed UCI: %q", call)
		}
		if strings.HasPrefix(call, "/etc/init.d/dnsmasq") {
			t.Errorf("GET /captive/status restarted dnsmasq: %q", call)
		}
		if strings.HasPrefix(call, "uci set dhcp.") || strings.HasPrefix(call, "uci delete dhcp.") ||
			strings.HasPrefix(call, "uci add_list dhcp.") {
			t.Errorf("GET /captive/status rewrote dnsmasq resolver options: %q", call)
		}
	}
	if _, err := os.Stat(guardFile); err != nil {
		t.Errorf("GET /captive/status consumed the bypass guard file: %v", err)
	}
}
