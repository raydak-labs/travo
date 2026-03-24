package services

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// mockAdGuardChecker is a test double for AdGuardChecker.
type mockAdGuardChecker struct {
	files        map[string]bool
	fileContents map[string]string
	commands     map[string]struct {
		output string
		err    error
	}
	httpGets map[string]struct {
		body []byte
		err  error
	}
	tcpProbes map[string]bool // addr -> reachable
}

func newMockAdGuardChecker() *mockAdGuardChecker {
	return &mockAdGuardChecker{
		files:        make(map[string]bool),
		fileContents: make(map[string]string),
		commands: make(map[string]struct {
			output string
			err    error
		}),
		httpGets: make(map[string]struct {
			body []byte
			err  error
		}),
		tcpProbes: make(map[string]bool),
	}
}

func (m *mockAdGuardChecker) FileExists(path string) bool {
	return m.files[path]
}

func (m *mockAdGuardChecker) RunCommand(name string, args ...string) (string, error) {
	key := name
	for _, a := range args {
		key += " " + a
	}
	if r, ok := m.commands[key]; ok {
		return r.output, r.err
	}
	return "", fmt.Errorf("command not mocked: %s", key)
}

func (m *mockAdGuardChecker) HTTPGet(url string) ([]byte, error) {
	if r, ok := m.httpGets[url]; ok {
		return r.body, r.err
	}
	return nil, fmt.Errorf("URL not mocked: %s", url)
}

func (m *mockAdGuardChecker) ReadFile(path string) ([]byte, error) {
	if content, ok := m.fileContents[path]; ok {
		return []byte(content), nil
	}
	if exists := m.files[path]; exists {
		return nil, fmt.Errorf("file %s exists but no content defined in mock", path)
	}
	return nil, os.ErrNotExist
}

func (m *mockAdGuardChecker) WriteFile(path string, data []byte, perm os.FileMode) error {
	m.fileContents[path] = string(data)
	m.files[path] = true
	return nil
}

func (m *mockAdGuardChecker) TCPProbe(addr string, _ time.Duration) bool {
	return m.tcpProbes[addr]
}

func TestIsInstalled_True(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.files["/opt/AdGuardHome/AdGuardHome"] = true
	svc := NewAdGuardServiceWithChecker(mock)

	if !svc.IsInstalled() {
		t.Error("expected IsInstalled=true when binary exists")
	}
}

func TestIsInstalled_False(t *testing.T) {
	mock := newMockAdGuardChecker()
	svc := NewAdGuardServiceWithChecker(mock)

	if svc.IsInstalled() {
		t.Error("expected IsInstalled=false when binary missing")
	}
}

func TestIsRunning_True(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.files["/etc/init.d/adguardhome"] = true
	mock.commands["/etc/init.d/adguardhome status"] = struct {
		output string
		err    error
	}{"running", nil}
	svc := NewAdGuardServiceWithChecker(mock)

	if !svc.IsRunning() {
		t.Error("expected IsRunning=true when service reports running")
	}
}

func TestIsRunning_False_NoInitScript(t *testing.T) {
	mock := newMockAdGuardChecker()
	svc := NewAdGuardServiceWithChecker(mock)

	if svc.IsRunning() {
		t.Error("expected IsRunning=false when no init script")
	}
}

func TestIsRunning_False_ServiceStopped(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.files["/etc/init.d/adguardhome"] = true
	mock.commands["/etc/init.d/adguardhome status"] = struct {
		output string
		err    error
	}{"", fmt.Errorf("exit status 1")}
	svc := NewAdGuardServiceWithChecker(mock)

	if svc.IsRunning() {
		t.Error("expected IsRunning=false when service not running")
	}
}

func TestVersion_Present(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.files["/opt/AdGuardHome/AdGuardHome"] = true
	mock.commands["/opt/AdGuardHome/AdGuardHome --version"] = struct {
		output string
		err    error
	}{"AdGuard Home, version v0.107.54", nil}
	svc := NewAdGuardServiceWithChecker(mock)

	v := svc.Version()
	if v != "v0.107.54" {
		t.Errorf("expected version 'v0.107.54', got %q", v)
	}
}

func TestVersion_NotInstalled(t *testing.T) {
	mock := newMockAdGuardChecker()
	svc := NewAdGuardServiceWithChecker(mock)

	if v := svc.Version(); v != "" {
		t.Errorf("expected empty version, got %q", v)
	}
}

func TestGetStatus_Success(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.httpGets["http://127.0.0.1:3000/control/status"] = struct {
		body []byte
		err  error
	}{
		body: []byte(`{"protection_enabled":true,"running":true,"version":"v0.107.54"}`),
	}
	mock.httpGets["http://127.0.0.1:3000/control/stats"] = struct {
		body []byte
		err  error
	}{
		body: []byte(`{"num_dns_queries":1000,"num_blocked_filtering":200,"num_replaced_safebrowsing":10,"num_replaced_parental":5,"avg_processing_time":0.025}`),
	}
	svc := NewAdGuardServiceWithChecker(mock)

	status, err := svc.GetStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Enabled {
		t.Error("expected Enabled=true")
	}
	if status.TotalQueries != 1000 {
		t.Errorf("expected TotalQueries=1000, got %d", status.TotalQueries)
	}
	if status.BlockedQueries != 215 {
		t.Errorf("expected BlockedQueries=215, got %d", status.BlockedQueries)
	}
	expectedPct := 21.5
	if status.BlockPercentage != expectedPct {
		t.Errorf("expected BlockPercentage=%.1f, got %.1f", expectedPct, status.BlockPercentage)
	}
	expectedMS := 25.0
	if status.AvgResponseMS != expectedMS {
		t.Errorf("expected AvgResponseMS=%.1f, got %.1f", expectedMS, status.AvgResponseMS)
	}
}

func TestGetStatus_APIUnreachable(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.httpGets["http://127.0.0.1:3000/control/status"] = struct {
		body []byte
		err  error
	}{
		err: fmt.Errorf("connection refused"),
	}
	svc := NewAdGuardServiceWithChecker(mock)

	_, err := svc.GetStatus()
	if err == nil {
		t.Error("expected error when API is unreachable")
	}
}

func TestGetDNSStatus_Enabled(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.httpGets["http://127.0.0.1:3000/control/dns_info"] = struct {
		body []byte
		err  error
	}{
		body: []byte(`{"port":5353}`),
	}
	mock.commands["uci get dhcp.@dnsmasq[0].server"] = struct {
		output string
		err    error
	}{"127.0.0.1#5353", nil}
	svc := NewAdGuardServiceWithChecker(mock)

	status, err := svc.GetDNSStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Enabled {
		t.Error("expected Enabled=true when server includes AdGuard entry")
	}
	if status.DNSPort != 5353 {
		t.Errorf("expected DNSPort=5353, got %d", status.DNSPort)
	}
}

func TestGetDNSStatus_Disabled(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.httpGets["http://127.0.0.1:3000/control/dns_info"] = struct {
		body []byte
		err  error
	}{
		body: []byte(`{"port":5353}`),
	}
	mock.commands["uci get dhcp.@dnsmasq[0].server"] = struct {
		output string
		err    error
	}{"", fmt.Errorf("uci: Entry not found")}
	svc := NewAdGuardServiceWithChecker(mock)

	status, err := svc.GetDNSStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Enabled {
		t.Error("expected Enabled=false when server option not set")
	}
}

func TestGetDNSStatus_DefaultPort(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.httpGets["http://127.0.0.1:3000/control/dns_info"] = struct {
		body []byte
		err  error
	}{
		err: fmt.Errorf("connection refused"),
	}
	mock.commands["uci get dhcp.@dnsmasq[0].server"] = struct {
		output string
		err    error
	}{"", fmt.Errorf("uci: Entry not found")}
	svc := NewAdGuardServiceWithChecker(mock)

	status, err := svc.GetDNSStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.DNSPort != 5353 {
		t.Errorf("expected default DNSPort=5353, got %d", status.DNSPort)
	}
}

func TestSetDNS_Enable(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.httpGets["http://127.0.0.1:3000/control/dns_info"] = struct {
		body []byte
		err  error
	}{
		body: []byte(`{"port":5353}`),
	}
	// Pre-flight: adguardhome init script exists (IsRunning checks this first).
	mock.files["/etc/init.d/adguardhome"] = true
	mock.commands["/etc/init.d/adguardhome status"] = struct {
		output string
		err    error
	}{"running", nil}
	// Pre-flight: DNS listener reachable on port 5353.
	mock.tcpProbes["127.0.0.1:5353"] = true
	// delete may fail (option not set yet), that's ok
	mock.commands["uci delete dhcp.@dnsmasq[0].server"] = struct {
		output string
		err    error
	}{"", nil}
	mock.commands["uci add_list dhcp.@dnsmasq[0].server=127.0.0.1#5353"] = struct {
		output string
		err    error
	}{"", nil}
	mock.commands["uci set dhcp.@dnsmasq[0].noresolv=1"] = struct {
		output string
		err    error
	}{"", nil}
	mock.commands["uci commit dhcp"] = struct {
		output string
		err    error
	}{"", nil}
	mock.commands["/etc/init.d/dnsmasq restart"] = struct {
		output string
		err    error
	}{"", nil}
	svc := NewAdGuardServiceWithChecker(mock)

	err := svc.SetDNS(true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetDNS_Disable(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.httpGets["http://127.0.0.1:3000/control/dns_info"] = struct {
		body []byte
		err  error
	}{
		body: []byte(`{"port":5353}`),
	}
	mock.commands["uci delete dhcp.@dnsmasq[0].server"] = struct {
		output string
		err    error
	}{"", nil}
	mock.commands["uci set dhcp.@dnsmasq[0].noresolv=0"] = struct {
		output string
		err    error
	}{"", nil}
	mock.commands["uci commit dhcp"] = struct {
		output string
		err    error
	}{"", nil}
	mock.commands["/etc/init.d/dnsmasq restart"] = struct {
		output string
		err    error
	}{"", nil}
	svc := NewAdGuardServiceWithChecker(mock)

	err := svc.SetDNS(false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetDNS_Enable_FailAddList(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.httpGets["http://127.0.0.1:3000/control/dns_info"] = struct {
		body []byte
		err  error
	}{
		body: []byte(`{"port":5353}`),
	}
	mock.commands["uci delete dhcp.@dnsmasq[0].server"] = struct {
		output string
		err    error
	}{"", nil}
	mock.commands["uci add_list dhcp.@dnsmasq[0].server=127.0.0.1#5353"] = struct {
		output string
		err    error
	}{"", fmt.Errorf("uci error")}
	svc := NewAdGuardServiceWithChecker(mock)

	err := svc.SetDNS(true)
	if err == nil {
		t.Error("expected error when add_list fails")
	}
}

func TestGetConfig_FileNotFound(t *testing.T) {
	mock := newMockAdGuardChecker()
	// Don't add the config file to the mock - it doesn't exist
	svc := NewAdGuardServiceWithChecker(mock)

	content, err := svc.GetConfig()
	if err == nil {
		t.Error("expected error when config file doesn't exist")
	}
	if content != "" {
		t.Error("expected empty content when file doesn't exist")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist error, got: %v", err)
	}
}

func TestGetConfig_Success(t *testing.T) {
	mock := newMockAdGuardChecker()
	testContent := `bind_host: 0.0.0.0
bind_port: 3000
users:
  - name: admin
    password: test`
	mock.fileContents["/opt/AdGuardHome/AdGuardHome.yaml"] = testContent
	svc := NewAdGuardServiceWithChecker(mock)

	content, err := svc.GetConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != testContent {
		t.Errorf("expected content %q, got %q", testContent, content)
	}
}

func TestSetConfig_Success(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.commands["/etc/init.d/adguardhome restart"] = struct {
		output string
		err    error
	}{"", nil}
	svc := NewAdGuardServiceWithChecker(mock)

	testContent := `bind_host: 0.0.0.0
bind_port: 3000`
	err := svc.SetConfig(testContent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the content was written
	if written, ok := mock.fileContents["/opt/AdGuardHome/AdGuardHome.yaml"]; !ok || written != testContent {
		t.Errorf("expected config to be written, got: %v", written)
	}
}

func TestIsInstalled_ViaRunningProcess(t *testing.T) {
	// Binary doesn't exist, init script doesn't exist,
	// but the process responds to HTTP (running via non-standard install).
	mock := newMockAdGuardChecker()
	mock.httpGets["http://127.0.0.1:3000/control/status"] = struct {
		body []byte
		err  error
	}{
		body: []byte(`{"running":true}`),
	}
	svc := NewAdGuardServiceWithChecker(mock)
	if !svc.IsInstalled() {
		t.Error("expected IsInstalled()=true when process is responding to API")
	}
}

func TestIsInstalled_ViaInitScript(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.files["/etc/init.d/adguardhome"] = true
	svc := NewAdGuardServiceWithChecker(mock)
	if !svc.IsInstalled() {
		t.Error("expected IsInstalled()=true when init script exists")
	}
}

func TestSetDNS_Enable_FailsWhenNotRunning(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.httpGets["http://127.0.0.1:3000/control/dns_info"] = struct {
		body []byte
		err  error
	}{
		body: []byte(`{"port":5353}`),
	}
	// AdGuard not running (no init script, no HTTP response)
	svc := NewAdGuardServiceWithChecker(mock)
	err := svc.SetDNS(true)
	if err == nil {
		t.Fatal("expected error when AdGuard not running")
	}
}

func TestSetDNS_Enable_FailsWhenListenerNotReady(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.httpGets["http://127.0.0.1:3000/control/dns_info"] = struct {
		body []byte
		err  error
	}{
		body: []byte(`{"port":5353}`),
	}
	mock.files["/etc/init.d/adguardhome"] = true
	mock.commands["/etc/init.d/adguardhome status"] = struct {
		output string
		err    error
	}{"running", nil}
	// TCP probe fails — DNS listener not ready.
	mock.tcpProbes["127.0.0.1:5353"] = false
	svc := NewAdGuardServiceWithChecker(mock)
	err := svc.SetDNS(true)
	if err == nil {
		t.Fatal("expected error when DNS listener not ready")
	}
}

func TestGetDNSStatus_HealthFields(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.httpGets["http://127.0.0.1:3000/control/dns_info"] = struct {
		body []byte
		err  error
	}{
		body: []byte(`{"port":5353}`),
	}
	mock.commands["uci get dhcp.@dnsmasq[0].server"] = struct {
		output string
		err    error
	}{"127.0.0.1#5353", nil}
	mock.tcpProbes["127.0.0.1:5353"] = true
	mock.commands["nslookup example.com 127.0.0.1"] = struct {
		output string
		err    error
	}{"Address: 93.184.216.34", nil}
	svc := NewAdGuardServiceWithChecker(mock)

	status, err := svc.GetDNSStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.AdguardListenerReady {
		t.Error("expected AdguardListenerReady=true")
	}
	if !status.ResolverProbeOk {
		t.Error("expected ResolverProbeOk=true")
	}
	if status.DnsmasqForwardTarget != "127.0.0.1#5353" {
		t.Errorf("expected DnsmasqForwardTarget='127.0.0.1#5353', got %q", status.DnsmasqForwardTarget)
	}
}

func TestAutoConfigureWritesDefaultConfig(t *testing.T) {
	mock := newMockAdGuardChecker()
	// No binary, no config file — fresh install.
	svc := NewAdGuardServiceWithChecker(mock)

	// mkdir and AdGuard start commands must not fail.
	mock.commands["mkdir -p /opt/AdGuardHome"] = struct{ output string; err error }{"", nil}

	err := svc.AutoConfigure()
	if err != nil {
		t.Fatalf("AutoConfigure: %v", err)
	}

	content, ok := mock.fileContents[adguardConfigPath]
	if !ok {
		t.Fatal("expected AdGuardHome.yaml to be written")
	}
	if !strings.Contains(content, "port: 5353") {
		t.Errorf("expected port 5353 in default config, got: %s", content)
	}
	if !strings.Contains(content, "bind_port: 3000") {
		t.Errorf("expected bind_port 3000 in default config, got: %s", content)
	}
}

func TestAutoConfigureSkipsExistingConfig(t *testing.T) {
	mock := newMockAdGuardChecker()
	// Config file already exists.
	mock.files[adguardConfigPath] = true
	mock.fileContents[adguardConfigPath] = "existing: config\n"
	svc := NewAdGuardServiceWithChecker(mock)

	_ = svc.AutoConfigure()

	// The existing content must be preserved.
	if content := mock.fileContents[adguardConfigPath]; content != "existing: config\n" {
		t.Errorf("expected existing config to be preserved, got: %q", content)
	}
}
