package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestDetectGatewayPortalURL_NoRoute(t *testing.T) {
	cmd := &mockCmdRunner{responses: map[string]string{
		"ip route show default": "",
	}}
	svc, dir := newTestCaptiveServiceWithUCI(&MockHTTPProber{StatusCode: 204}, cmd)
	defer os.RemoveAll(dir)

	got := svc.detectGatewayPortalURL()
	if got != "" {
		t.Errorf("detectGatewayPortalURL with no route = %q, want empty", got)
	}
}

func TestDetectGatewayPortalURL_NoCmd(t *testing.T) {
	svc := NewCaptiveService(&MockHTTPProber{StatusCode: 204})

	got := svc.detectGatewayPortalURL()
	if got != "" {
		t.Errorf("detectGatewayPortalURL with no cmd = %q, want empty", got)
	}
}

// newMockAdGuardServer returns a test HTTP server that mimics the AdGuardHome
// /control/dns_info and /control/dns_config endpoints.
func newMockAdGuardServer(t *testing.T, upstream []string) (server *httptest.Server, receivedUpstream *[]string) {
	t.Helper()
	received := &[]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/control/dns_info":
			_ = json.NewEncoder(w).Encode(adguardDNSInfoResp{UpstreamDNS: upstream})
		case "/control/dns_config":
			var cfg adguardDNSConfig
			_ = json.NewDecoder(r.Body).Decode(&cfg)
			*received = cfg.UpstreamDNS
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return srv, received
}

func newTestCaptiveServiceWithAdGuard(t *testing.T, cmd *mockCmdRunner, agServer *httptest.Server) (*CaptiveService, string) {
	t.Helper()
	u := uci.NewMockUCI()
	guardDir, _ := os.MkdirTemp("", "captive-test-*")
	guardFile := filepath.Join(guardDir, "dns-bypass-in-progress")
	svc := &CaptiveService{prober: &MockHTTPProber{StatusCode: 204}, uci: u, cmd: cmd, guardFile: guardFile}
	if agServer != nil {
		// Patch the package-level constant via a subfield isn't possible directly,
		// so we test via the exported methods with an overridden API base.
		// We use a helper that replaces adguardAPIBase for the scope of the test.
		svc.adguardAPIBaseOverride = agServer.URL
	}
	return svc, guardDir
}

// TestBypassDNS_WithAdGuard verifies that BypassDNS patches AdGuardHome's upstream
// when AdGuardHome is present and using encrypted DNS.
func TestBypassDNS_WithAdGuard_EncryptedUpstream(t *testing.T) {
	agSrv, received := newMockAdGuardServer(t, []string{"https://dns10.quad9.net/dns-query"})
	defer agSrv.Close()

	// Write a fake resolv.conf.auto so readDHCPDNS finds a hotel DNS
	tmpResolvDir, _ := os.MkdirTemp("", "resolv-*")
	defer os.RemoveAll(tmpResolvDir)
	resolvFile := filepath.Join(tmpResolvDir, "resolv.conf.auto")
	_ = os.WriteFile(resolvFile, []byte("# test\nnameserver 10.1.2.3\n"), 0600)

	cmd := &mockCmdRunner{responses: map[string]string{
		"uci get dhcp.@dnsmasq[0].noresolv":          "0",
		"uci get dhcp.@dnsmasq[0].server":            "",
		"uci get dhcp.@dnsmasq[0].rebind_protection": "0",
	}}
	svc, dir := newTestCaptiveServiceWithAdGuard(t, cmd, agSrv)
	defer os.RemoveAll(dir)
	svc.resolvConfAutoOverride = resolvFile
	_ = svc.uci.Set("network", "wan", "peerdns", "1")

	if err := svc.BypassDNS(); err != nil {
		t.Fatalf("BypassDNS error: %v", err)
	}
	if !svc.IsDNSBypassed() {
		t.Error("expected guard file to exist after AdGuardHome-only bypass")
	}
	if len(*received) == 0 || (*received)[0] != "10.1.2.3" {
		t.Errorf("expected AdGuardHome upstream set to 10.1.2.3, got %v", *received)
	}
}

// TestBypassDNS_NoAdGuard verifies that BypassDNS still works (dnsmasq path)
// when AdGuardHome is not installed (no HTTP server at adguardAPIBase).
func TestBypassDNS_NoAdGuard_DnsmasqOnly(t *testing.T) {
	cmd := &mockCmdRunner{responses: map[string]string{
		"uci get dhcp.@dnsmasq[0].noresolv":          "1",
		"uci get dhcp.@dnsmasq[0].server":            "127.0.0.1#5353",
		"uci get dhcp.@dnsmasq[0].rebind_protection": "0",
		"uci set dhcp.@dnsmasq[0].noresolv=0":        "",
		"uci delete dhcp.@dnsmasq[0].server":         "",
		"uci add_list dhcp.@dnsmasq[0].server=":      "", // hotel DNS empty (no resolv.conf.auto in test)
		"uci commit dhcp":                            "",
		"/etc/init.d/dnsmasq restart":                "",
	}}
	// No AdGuard server — calls to adguardAPIBase will fail with connection refused
	svc, dir := newTestCaptiveServiceWithAdGuard(t, cmd, nil)
	defer os.RemoveAll(dir)
	// Point adguardAPIBaseOverride to a non-listening address
	svc.adguardAPIBaseOverride = "http://127.0.0.1:19999"
	_ = svc.uci.Set("network", "wan", "peerdns", "1")

	if err := svc.BypassDNS(); err != nil {
		t.Fatalf("BypassDNS should succeed even without AdGuardHome: %v", err)
	}
	if !svc.IsDNSBypassed() {
		t.Error("expected guard file to exist (dnsmasq-only bypass)")
	}
}

// TestCheckDNSBypassNeeded_AdGuardEncrypted verifies that CheckDNSBypassNeeded
// returns true when AdGuardHome is running with encrypted upstreams.
func TestCheckDNSBypassNeeded_AdGuardEncrypted(t *testing.T) {
	agSrv, _ := newMockAdGuardServer(t, []string{"https://dns10.quad9.net/dns-query"})
	defer agSrv.Close()

	cmd := &mockCmdRunner{responses: map[string]string{
		"uci get dhcp.@dnsmasq[0].noresolv": "0",
		"uci get dhcp.@dnsmasq[0].server":   "",
	}}
	svc, dir := newTestCaptiveServiceWithAdGuard(t, cmd, agSrv)
	defer os.RemoveAll(dir)
	svc.adguardAPIBaseOverride = agSrv.URL
	_ = svc.uci.Set("network", "wan", "peerdns", "1")

	if !svc.CheckDNSBypassNeeded() {
		t.Error("expected bypass needed when AdGuardHome uses encrypted DNS")
	}
}

// TestCheckDNSBypassNeeded_AdGuardPlainDNS verifies bypass is not reported
// needed when AdGuardHome uses plain IP upstreams (hotel DNS already).
func TestCheckDNSBypassNeeded_AdGuardPlainDNS(t *testing.T) {
	agSrv, _ := newMockAdGuardServer(t, []string{"10.1.2.3"})
	defer agSrv.Close()

	cmd := &mockCmdRunner{responses: map[string]string{
		"uci get dhcp.@dnsmasq[0].noresolv": "0",
		"uci get dhcp.@dnsmasq[0].server":   "",
	}}
	svc, dir := newTestCaptiveServiceWithAdGuard(t, cmd, agSrv)
	defer os.RemoveAll(dir)
	svc.adguardAPIBaseOverride = agSrv.URL
	_ = svc.uci.Set("network", "wan", "peerdns", "1")

	if svc.CheckDNSBypassNeeded() {
		t.Error("expected no bypass needed when AdGuardHome already uses plain IP upstream")
	}
}
