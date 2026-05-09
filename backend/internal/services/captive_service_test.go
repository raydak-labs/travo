package services

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestCaptivePortal_NoRedirect_NoPortal(t *testing.T) {
	prober := &MockHTTPProber{
		StatusCode: 204,
		Body:       "",
	}
	svc := NewCaptiveService(prober)

	status, err := svc.CheckCaptivePortal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Detected {
		t.Error("expected no captive portal detected")
	}
	if !status.CanReachInternet {
		t.Error("expected internet reachable")
	}
	if status.PortalURL != nil {
		t.Error("expected no portal URL")
	}
}

func TestCaptivePortal_Redirect_PortalDetected(t *testing.T) {
	prober := &MockHTTPProber{
		StatusCode:  302,
		Body:        "",
		RedirectURL: "http://portal.hotel.com/login",
	}
	svc := NewCaptiveService(prober)

	status, err := svc.CheckCaptivePortal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Detected {
		t.Error("expected captive portal detected")
	}
	if status.CanReachInternet {
		t.Error("expected internet NOT reachable")
	}
	if status.PortalURL == nil {
		t.Fatal("expected portal URL")
	}
	if *status.PortalURL != "http://portal.hotel.com/login" {
		t.Errorf("expected portal URL 'http://portal.hotel.com/login', got %q", *status.PortalURL)
	}
}

func TestCaptivePortal_ConnectionFailed_NoInternet(t *testing.T) {
	prober := &MockHTTPProber{
		Err: fmt.Errorf("dial tcp: connection refused"),
	}
	svc := NewCaptiveService(prober)

	status, err := svc.CheckCaptivePortal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Detected {
		t.Error("expected no captive portal detected on connection failure")
	}
	if status.CanReachInternet {
		t.Error("expected internet NOT reachable")
	}
}

func TestCaptivePortal_WrongBody_PortalDetected(t *testing.T) {
	prober := &MockHTTPProber{
		StatusCode: 200,
		Body:       "<html><body>Please login to continue</body></html>",
	}
	svc := NewCaptiveService(prober)

	status, err := svc.CheckCaptivePortal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Detected {
		t.Error("expected captive portal detected when body is wrong")
	}
	if status.CanReachInternet {
		t.Error("expected internet NOT reachable")
	}
	if status.PortalURL == nil {
		t.Fatal("expected fallback portal URL for HTTP 200 captive portal")
	}
	if *status.PortalURL != "http://connectivitycheck.gstatic.com/generate_204" {
		t.Errorf("expected probe URL as fallback, got %q", *status.PortalURL)
	}
}

func TestCaptivePortal_301Redirect(t *testing.T) {
	prober := &MockHTTPProber{
		StatusCode:  301,
		Body:        "",
		RedirectURL: "http://captive.example.com",
	}
	svc := NewCaptiveService(prober)

	status, err := svc.CheckCaptivePortal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Detected {
		t.Error("expected captive portal detected on 301")
	}
}

func TestCaptivePortal_307Redirect(t *testing.T) {
	prober := &MockHTTPProber{
		StatusCode:  307,
		Body:        "",
		RedirectURL: "http://captive.example.com",
	}
	svc := NewCaptiveService(prober)

	status, err := svc.CheckCaptivePortal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Detected {
		t.Error("expected captive portal detected on 307")
	}
}

// mockCmdRunner records calls and returns preset responses.
type mockCmdRunner struct {
	responses map[string]string
	calls     [][]string
}

func (m *mockCmdRunner) Run(name string, args ...string) ([]byte, error) {
	call := append([]string{name}, args...)
	m.calls = append(m.calls, call)
	key := name
	for _, a := range args {
		key += " " + a
	}
	if resp, ok := m.responses[key]; ok {
		return []byte(resp), nil
	}
	return nil, fmt.Errorf("command not found: %s", key)
}

func newTestCaptiveServiceWithUCI(prober HTTPProber, cmd *mockCmdRunner) (*CaptiveService, string) {
	u := uci.NewMockUCI()
	guardDir, _ := os.MkdirTemp("", "captive-test-*")
	guardFile := filepath.Join(guardDir, "dns-bypass-in-progress")
	svc := &CaptiveService{prober: prober, uci: u, cmd: cmd, guardFile: guardFile}
	return svc, guardDir
}

func TestIsDNSBypassed_NoGuardFile(t *testing.T) {
	svc, dir := newTestCaptiveServiceWithUCI(&MockHTTPProber{StatusCode: 204}, &mockCmdRunner{responses: map[string]string{}})
	defer os.RemoveAll(dir)

	if svc.IsDNSBypassed() {
		t.Error("expected not bypassed when no guard file")
	}
}

func TestBypassDNS_CreatesGuardFile(t *testing.T) {
	cmd := &mockCmdRunner{responses: map[string]string{
		"uci get dhcp.@dnsmasq[0].noresolv":            "1",
		"uci get dhcp.@dnsmasq[0].server":              "127.0.0.1#5353",
		"uci get dhcp.@dnsmasq[0].rebind_protection":   "1",
		"uci set dhcp.@dnsmasq[0].noresolv=0":          "",
		"uci delete dhcp.@dnsmasq[0].server":           "",
		"uci set dhcp.@dnsmasq[0].rebind_protection=0": "",
		"uci commit dhcp":                              "",
		"/etc/init.d/dnsmasq restart":                  "",
	}}
	svc, dir := newTestCaptiveServiceWithUCI(&MockHTTPProber{StatusCode: 204}, cmd)
	defer os.RemoveAll(dir)

	err := svc.BypassDNS()
	if err != nil {
		t.Fatalf("BypassDNS error: %v", err)
	}
	if !svc.IsDNSBypassed() {
		t.Error("expected guard file to exist after bypass")
	}
}

func TestBypassDNS_NothingToBypass(t *testing.T) {
	cmd := &mockCmdRunner{responses: map[string]string{
		"uci get dhcp.@dnsmasq[0].noresolv":          "0",
		"uci get dhcp.@dnsmasq[0].server":            "",
		"uci get dhcp.@dnsmasq[0].rebind_protection": "0",
	}}
	svc, dir := newTestCaptiveServiceWithUCI(&MockHTTPProber{StatusCode: 204}, cmd)
	defer os.RemoveAll(dir)

	// Set wan peerdns to something that doesn't trigger bypass
	_ = svc.uci.Set("network", "wan", "peerdns", "1")

	err := svc.BypassDNS()
	if err != nil {
		t.Fatalf("BypassDNS error: %v", err)
	}
	if svc.IsDNSBypassed() {
		t.Error("should not create guard file when nothing to bypass")
	}
}

func TestRestoreDNS_RestoresAndRemovesGuard(t *testing.T) {
	cmd := &mockCmdRunner{responses: map[string]string{
		"uci get dhcp.@dnsmasq[0].noresolv":                   "1",
		"uci get dhcp.@dnsmasq[0].server":                     "127.0.0.1#5353",
		"uci get dhcp.@dnsmasq[0].rebind_protection":          "1",
		"uci set dhcp.@dnsmasq[0].noresolv=0":                 "",
		"uci delete dhcp.@dnsmasq[0].server":                  "",
		"uci set dhcp.@dnsmasq[0].rebind_protection=0":        "",
		"uci commit dhcp":                                     "",
		"/etc/init.d/dnsmasq restart":                         "",
		"uci set dhcp.@dnsmasq[0].noresolv=1":                 "",
		"uci add_list dhcp.@dnsmasq[0].server=127.0.0.1#5353": "",
		"uci set dhcp.@dnsmasq[0].rebind_protection=1":        "",
	}}
	svc, dir := newTestCaptiveServiceWithUCI(&MockHTTPProber{StatusCode: 204}, cmd)
	defer os.RemoveAll(dir)

	// First bypass
	if err := svc.BypassDNS(); err != nil {
		t.Fatalf("BypassDNS: %v", err)
	}
	if !svc.IsDNSBypassed() {
		t.Fatal("expected bypassed")
	}

	// Then restore
	if err := svc.RestoreDNS(); err != nil {
		t.Fatalf("RestoreDNS: %v", err)
	}
	if svc.IsDNSBypassed() {
		t.Error("expected guard file removed after restore")
	}
}

func TestRestoreDNS_NoGuardFile_Noop(t *testing.T) {
	svc, dir := newTestCaptiveServiceWithUCI(&MockHTTPProber{StatusCode: 204}, &mockCmdRunner{responses: map[string]string{}})
	defer os.RemoveAll(dir)

	err := svc.RestoreDNS()
	if err != nil {
		t.Fatalf("RestoreDNS should be noop when no guard file: %v", err)
	}
}

func TestMaybeAutoRestoreDNS_RestoresWhenInternetReachable(t *testing.T) {
	cmd := &mockCmdRunner{responses: map[string]string{
		"uci get dhcp.@dnsmasq[0].noresolv":                   "1",
		"uci get dhcp.@dnsmasq[0].server":                     "127.0.0.1#5353",
		"uci get dhcp.@dnsmasq[0].rebind_protection":          "0",
		"uci set dhcp.@dnsmasq[0].noresolv=0":                 "",
		"uci delete dhcp.@dnsmasq[0].server":                  "",
		"uci commit dhcp":                                     "",
		"/etc/init.d/dnsmasq restart":                         "",
		"uci set dhcp.@dnsmasq[0].noresolv=1":                 "",
		"uci add_list dhcp.@dnsmasq[0].server=127.0.0.1#5353": "",
	}}
	svc, dir := newTestCaptiveServiceWithUCI(&MockHTTPProber{StatusCode: 204}, cmd)
	defer os.RemoveAll(dir)

	_ = svc.BypassDNS()
	if !svc.IsDNSBypassed() {
		t.Fatal("expected bypassed")
	}

	svc.MaybeAutoRestoreDNS(true)
	if svc.IsDNSBypassed() {
		t.Error("expected auto-restore when internet reachable")
	}
}

func TestMaybeAutoRestoreDNS_NoRestoreWhenNoInternet(t *testing.T) {
	cmd := &mockCmdRunner{responses: map[string]string{
		"uci get dhcp.@dnsmasq[0].noresolv":          "1",
		"uci get dhcp.@dnsmasq[0].server":            "127.0.0.1#5353",
		"uci get dhcp.@dnsmasq[0].rebind_protection": "0",
		"uci set dhcp.@dnsmasq[0].noresolv=0":        "",
		"uci delete dhcp.@dnsmasq[0].server":         "",
		"uci commit dhcp":                            "",
		"/etc/init.d/dnsmasq restart":                "",
	}}
	svc, dir := newTestCaptiveServiceWithUCI(&MockHTTPProber{StatusCode: 204}, cmd)
	defer os.RemoveAll(dir)

	_ = svc.BypassDNS()
	svc.MaybeAutoRestoreDNS(false)
	if !svc.IsDNSBypassed() {
		t.Error("should NOT restore when internet not reachable")
	}
}

func TestCheckDNSBypassNeeded_AdGuard(t *testing.T) {
	cmd := &mockCmdRunner{responses: map[string]string{
		"uci get dhcp.@dnsmasq[0].noresolv": "1",
		"uci get dhcp.@dnsmasq[0].server":   "127.0.0.1#5353",
	}}
	svc, dir := newTestCaptiveServiceWithUCI(&MockHTTPProber{StatusCode: 204}, cmd)
	defer os.RemoveAll(dir)

	if !svc.CheckDNSBypassNeeded() {
		t.Error("expected bypass needed with noresolv=1 and custom server")
	}
}

func TestCheckDNSBypassNeeded_NoCustomDNS(t *testing.T) {
	cmd := &mockCmdRunner{responses: map[string]string{
		"uci get dhcp.@dnsmasq[0].noresolv": "0",
		"uci get dhcp.@dnsmasq[0].server":   "",
	}}
	svc, dir := newTestCaptiveServiceWithUCI(&MockHTTPProber{StatusCode: 204}, cmd)
	defer os.RemoveAll(dir)

	_ = svc.uci.Set("network", "wan", "peerdns", "1")

	if svc.CheckDNSBypassNeeded() {
		t.Error("expected no bypass needed with default DNS settings")
	}
}

func TestDetectGatewayPortalURL(t *testing.T) {
	cmd := &mockCmdRunner{responses: map[string]string{
		"ip route show default": "default via 192.168.1.1 dev eth0",
	}}
	prober := &MockHTTPProber{
		StatusCode:  302,
		RedirectURL: "http://192.168.1.1/portal",
	}
	svc, dir := newTestCaptiveServiceWithUCI(prober, cmd)
	defer os.RemoveAll(dir)

	got := svc.detectGatewayPortalURL()
	// detectGatewayPortalURL returns the gateway URL (not the redirect target)
	if got != "http://192.168.1.1/" {
		t.Errorf("detectGatewayPortalURL = %q, want http://192.168.1.1/", got)
	}
}
