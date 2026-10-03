package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// mockAdGuardChecker is a test double for AdGuardChecker.
type mockAdGuardChecker struct {
	mu           sync.Mutex
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
	// tcpProbeSeq lets a test make the listener appear only after N probes, to
	// exercise SetConfig's "came up late" path.
	tcpProbeSeq map[string][]bool
	// writeErr, when set, makes WriteFile fail for the matching path.
	writeErr map[string]error
	// addListRecorder, when set, captures every `uci add_list
	// dhcp.@dnsmasq[0].server=<entry>` value. The entries are dynamic, so they
	// cannot be pre-registered in the commands map.
	addListRecorder func(entry string)
	// calls records every command the service ran, so a test can assert on
	// absence as well as presence.
	calls []string
}

// recordedCalls returns the commands RunCommand has seen so far.
func (m *mockAdGuardChecker) recordedCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.calls)
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
		tcpProbes:   make(map[string]bool),
		tcpProbeSeq: make(map[string][]bool),
		writeErr:    make(map[string]error),
	}
}

func (m *mockAdGuardChecker) FileExists(path string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.files[path]
}

func (m *mockAdGuardChecker) RunCommand(name string, args ...string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := name
	for _, a := range args {
		key += " " + a
	}
	m.calls = append(m.calls, key)
	if r, ok := m.commands[key]; ok {
		return r.output, r.err
	}
	const addListPrefix = "uci add_list dhcp.@dnsmasq[0].server="
	if m.addListRecorder != nil && strings.HasPrefix(key, addListPrefix) {
		m.addListRecorder(strings.TrimPrefix(key, addListPrefix))
		return "", nil
	}
	return "", fmt.Errorf("command not mocked: %s", key)
}

func (m *mockAdGuardChecker) HTTPGet(url string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.httpGets[url]; ok {
		return r.body, r.err
	}
	return nil, fmt.Errorf("URL not mocked: %s", url)
}

func (m *mockAdGuardChecker) ReadFile(path string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if content, ok := m.fileContents[path]; ok {
		return []byte(content), nil
	}
	if exists := m.files[path]; exists {
		return nil, fmt.Errorf("file %s exists but no content defined in mock", path)
	}
	return nil, os.ErrNotExist
}

func (m *mockAdGuardChecker) WriteFile(path string, data []byte, perm os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.writeErr[path]; err != nil {
		return err
	}
	m.fileContents[path] = string(data)
	m.files[path] = true
	return nil
}

func (m *mockAdGuardChecker) RemoveFile(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.fileContents, path)
	delete(m.files, path)
	return nil
}

func (m *mockAdGuardChecker) TCPProbe(addr string, _ time.Duration) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if seq, ok := m.tcpProbeSeq[addr]; ok {
		if len(seq) == 0 {
			return false
		}
		next := seq[0]
		m.tcpProbeSeq[addr] = seq[1:]
		return next
	}
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
	// The DNS listener must come back for the write to be accepted.
	mock.tcpProbes["127.0.0.1:5353"] = true
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
	mock.commands["mkdir -p /opt/AdGuardHome"] = struct {
		output string
		err    error
	}{"", nil}

	err := svc.AutoConfigure()
	if err != nil {
		t.Fatalf("AutoConfigure: %v", err)
	}

	content, ok := mock.fileContents[adguardYAMLPathOpt]
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
	mock.files[adguardYAMLPathOpt] = true
	mock.fileContents[adguardYAMLPathOpt] = "existing: config\n"
	svc := NewAdGuardServiceWithChecker(mock)

	_ = svc.AutoConfigure()

	// The existing content must be preserved.
	if content := mock.fileContents[adguardYAMLPathOpt]; content != "existing: config\n" {
		t.Errorf("expected existing config to be preserved, got: %q", content)
	}
}

// mockAdGuardDNSChecker builds a mock with everything SetDNS(true) needs:
// AdGuard running, its DNS listener reachable, and every dnsmasq mutation
// accepted. initialServers is what dnsmasq reports for dhcp.@dnsmasq[0].server
// before the toggle.
func mockAdGuardDNSChecker(initialServers []string) *mockAdGuardChecker {
	m := newMockAdGuardChecker()
	m.files[adguardInitd] = true
	m.commands[adguardInitd+" status"] = struct {
		output string
		err    error
	}{"running", nil}
	m.tcpProbes["127.0.0.1:5353"] = true
	m.commands["uci get dhcp.@dnsmasq[0].server"] = struct {
		output string
		err    error
	}{strings.Join(initialServers, " "), nil}
	m.commands["uci get dhcp.@dnsmasq[0].noresolv"] = struct {
		output string
		err    error
	}{"", nil}
	for _, cmd := range []string{
		"uci delete dhcp.@dnsmasq[0].server",
		"uci set dhcp.@dnsmasq[0].noresolv=1",
		"uci set dhcp.@dnsmasq[0].noresolv=0",
		"uci commit dhcp",
		"/etc/init.d/dnsmasq restart",
	} {
		m.commands[cmd] = struct {
			output string
			err    error
		}{"", nil}
	}
	return m
}

// newStackScopedAdGuard returns a service whose shared dnsmasq layer record
// lives in the mock's own file map, so the stack lifecycle can be asserted.
func newStackScopedAdGuard(t *testing.T, mock *mockAdGuardChecker) *AdGuardService {
	t.Helper()
	svc := NewAdGuardServiceWithChecker(mock)
	svc.dnsStackPath = "dnsmasq-layers.json"
	return svc
}

// readAdGuardLayerStack reads back the shared record through the mock's files.
func readAdGuardLayerStack(t *testing.T, svc *AdGuardService) *dnsmasqLayerStack {
	t.Helper()
	st, err := svc.dnsmasqLayers().load()
	if err != nil {
		t.Fatalf("reading the layer stack: %v", err)
	}
	if st == nil {
		t.Fatal("expected a dnsmasq layer stack on disk, got none")
	}
	return st
}

// sharedDnsmasqChecker is an AdGuardChecker whose `uci` calls drive the same
// dhcp.@dnsmasq[0] model and temp-dir files a VpnService sees, so the two
// features can be stacked on one record in a cross-feature test. Everything
// that is not a dnsmasq call goes to the embedded mock.
type sharedDnsmasqChecker struct {
	*mockAdGuardChecker
	state *dnsmasqState
	dir   string
}

func (c *sharedDnsmasqChecker) RunCommand(name string, args ...string) (string, error) {
	if name == "uci" {
		out, err := c.state.runUci(args...)
		return string(out), err
	}
	return c.mockAdGuardChecker.RunCommand(name, args...)
}

func (c *sharedDnsmasqChecker) resolve(path string) string {
	return filepath.Join(c.dir, path)
}

func (c *sharedDnsmasqChecker) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(c.resolve(path))
}

func (c *sharedDnsmasqChecker) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(c.resolve(path), data, perm)
}

func (c *sharedDnsmasqChecker) RemoveFile(path string) error {
	if err := os.Remove(c.resolve(path)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// sharedDNSHarness pairs an AdGuardService and a VpnService over one dnsmasq
// model and one shared layer record, which is the situation the record exists
// for: both features write the same two options of the same section.
type sharedDNSHarness struct {
	t         *testing.T
	adguard   *AdGuardService
	vpn       *VpnService
	state     *dnsmasqState
	stackPath string
}

// newSharedDNSHarness starts from a router whose LAN DNS points at 1.1.1.1 and
// whose tunnel is configured with 10.8.0.1 as its resolver.
func newSharedDNSHarness(t *testing.T) *sharedDNSHarness {
	t.Helper()
	cmd, state := newDnsmasqRunner([]string{"1.1.1.1"}, true)
	dir := t.TempDir()
	stackPath := filepath.Join(dir, "dnsmasq-layers.json")

	checker := &sharedDnsmasqChecker{
		mockAdGuardChecker: mockAdGuardDNSChecker(nil),
		state:              state,
		dir:                dir,
	}
	adguard := NewAdGuardServiceWithChecker(checker)
	// Relative: the checker resolves it under dir, which is the same file the
	// VpnService is pointed at with an absolute path.
	adguard.dnsStackPath = "dnsmasq-layers.json"

	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0", "dns", "10.8.0.1")
	_ = u.Set("network", "wg0", "disabled", "0")
	vpn := NewVpnServiceWithRunner(u, cmd)
	vpn.dnsStackPath = stackPath
	vpn.legacyDnsSnapshotPath = filepath.Join(dir, "vpn-dns-snapshot.json")

	return &sharedDNSHarness{
		t:         t,
		adguard:   adguard,
		vpn:       vpn,
		state:     state,
		stackPath: stackPath,
	}
}

// adguardEnabled turns AdGuard forwarding on through the public entry point.
func (h *sharedDNSHarness) adguardEnabled() {
	h.t.Helper()
	if err := h.adguard.SetDNS(true); err != nil {
		h.t.Fatalf("enable AdGuard DNS: %v", err)
	}
}

// readStack reads the shared record straight off disk, so the assertion does
// not depend on either service's own view of it.
func (h *sharedDNSHarness) readStack(t *testing.T) *dnsmasqLayerStack {
	t.Helper()
	data, err := os.ReadFile(h.stackPath)
	if err != nil {
		t.Fatalf("reading the shared dnsmasq record: %v", err)
	}
	var st dnsmasqLayerStack
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("parsing the shared dnsmasq record: %v", err)
	}
	return &st
}

// A split-DNS entry the operator added in LuCI must survive an AdGuard
// enable/disable round trip. Before the record existed, SetDNS(false)
// `uci delete`d the whole server list and the entry was gone for good.
func TestSetDNS_RoundTripPreservesSplitDNSEntry(t *testing.T) {
	const splitEntry = "server=/lan.example.com/192.168.9.5"
	mock := mockAdGuardDNSChecker([]string{splitEntry})
	var added []string
	mock.addListRecorder = func(entry string) { added = append(added, entry) }
	svc := newStackScopedAdGuard(t, mock)

	if err := svc.SetDNS(true); err != nil {
		t.Fatalf("SetDNS(true): %v", err)
	}
	st := readAdGuardLayerStack(t, svc)
	if len(st.Servers) != 1 || st.Servers[0] != splitEntry {
		t.Fatalf("enable must record the pre-existing split-DNS entry, got %v", st.Servers)
	}
	if len(st.Layers) != 1 || st.Layers[0].Name != dnsLayerAdGuard {
		t.Fatalf("enable must stack the adguard layer, got %+v", st.Layers)
	}

	added = nil
	if err := svc.SetDNS(false); err != nil {
		t.Fatalf("SetDNS(false): %v", err)
	}

	if len(added) != 1 || added[0] != splitEntry {
		t.Fatalf("disable must restore exactly the recorded server list, got %v", added)
	}
	if _, ok := mock.fileContents["dnsmasq-layers.json"]; ok {
		t.Error("the record must be removed once the last layer is restored")
	}
}

// A second enable while forwarding is already on must not re-record AdGuard's
// own entry as the pre-any-layer list.
func TestSetDNS_SecondEnableKeepsFirstSnapshot(t *testing.T) {
	mock := mockAdGuardDNSChecker([]string{"1.1.1.1"})
	mock.addListRecorder = func(string) {}
	svc := newStackScopedAdGuard(t, mock)

	if err := svc.SetDNS(true); err != nil {
		t.Fatalf("first SetDNS(true): %v", err)
	}
	first := readAdGuardLayerStack(t, svc)
	if len(first.Servers) != 1 || first.Servers[0] != "1.1.1.1" {
		t.Fatalf("the record must hold the pre-layer server list, got %v", first.Servers)
	}

	// Second enable: dnsmasq now reports AdGuard's own entry.
	mock.mu.Lock()
	mock.commands["uci get dhcp.@dnsmasq[0].server"] = struct {
		output string
		err    error
	}{"127.0.0.1#5353", nil}
	mock.mu.Unlock()
	if err := svc.SetDNS(true); err != nil {
		t.Fatalf("second SetDNS(true): %v", err)
	}
	if got := readAdGuardLayerStack(t, svc); !slices.Equal(got.Servers, first.Servers) {
		t.Fatalf("the second enable must not overwrite the base:\nfirst: %v\nsecond: %v",
			first.Servers, got.Servers)
	}
}

// A record that cannot be written must abort the enable before dnsmasq is
// touched: there would be nothing to restore afterwards.
func TestSetDNS_EnableFailsWhenSnapshotWriteFails(t *testing.T) {
	mock := mockAdGuardDNSChecker([]string{"1.1.1.1"})
	mock.writeErr["dnsmasq-layers.json"] = errors.New("read-only file system")
	svc := newStackScopedAdGuard(t, mock)

	if err := svc.SetDNS(true); err == nil {
		t.Fatal("expected SetDNS(true) to fail when the record cannot be written")
	}
	for _, call := range mock.recordedCalls() {
		if call == "uci commit dhcp" || call == "/etc/init.d/dnsmasq restart" {
			t.Errorf("dnsmasq must not be touched when the record failed, but %q was called", call)
		}
	}
}

// With no record, disable clears noresolv but leaves the server list alone: it
// may be entirely the operator's own split-DNS entries.
func TestSetDNS_DisableWithoutSnapshotKeepsServerList(t *testing.T) {
	mock := mockAdGuardDNSChecker([]string{"1.1.1.1"})
	mock.addListRecorder = func(string) {}
	svc := newStackScopedAdGuard(t, mock)

	if err := svc.SetDNS(false); err != nil {
		t.Fatalf("SetDNS(false): %v", err)
	}
	for _, call := range mock.recordedCalls() {
		if call == "uci delete dhcp.@dnsmasq[0].server" {
			t.Error("disable without a record must not delete the dnsmasq server list")
		}
	}
	if !slices.Contains(mock.recordedCalls(), "uci set dhcp.@dnsmasq[0].noresolv=0") {
		t.Error("disable without a record must still clear noresolv")
	}
}

// A device upgraded while AdGuard forwarding was on has only the pre-stack
// snapshot. The enable must adopt it as the base and then drop it, so there is
// one restore target for dnsmasq's resolver options instead of two.
func TestSetDNS_EnableAdoptsAndRemovesTheLegacySnapshot(t *testing.T) {
	const splitEntry = "server=/lan.example.com/192.168.9.5"
	mock := mockAdGuardDNSChecker([]string{"127.0.0.1#5353"})
	mock.addListRecorder = func(string) {}
	mock.fileContents[adguardDnsSnapshotPath] = `{"noresolv":"0","servers":["` + splitEntry + `"]}`
	mock.files[adguardDnsSnapshotPath] = true
	svc := newStackScopedAdGuard(t, mock)

	if err := svc.SetDNS(true); err != nil {
		t.Fatalf("SetDNS(true): %v", err)
	}
	if _, ok := mock.fileContents[adguardDnsSnapshotPath]; ok {
		t.Error("the legacy snapshot must be dropped once the stack owns the base")
	}
	st := readAdGuardLayerStack(t, svc)
	if len(st.Servers) != 1 || st.Servers[0] != splitEntry {
		t.Fatalf("the legacy snapshot must become the stack base, got %v", st.Servers)
	}
}

// The cross-feature damage the shared stack exists to prevent: AdGuard and VPN
// forward to the same two options of the same dnsmasq section, so with separate
// records the last restore landed on top of the other's state and the operator
// was left with dnsmasq pointing at the router itself.
//
// AdGuard is enabled first, then the VPN on top of it. Turning AdGuard off
// leaves the VPN running and dnsmasq untouched; turning the VPN off restores
// the pre-any-layer list and drops the record.
func TestDNSLayerStack_AdGuardUnderVPNRestoresInOrder(t *testing.T) {
	h := newSharedDNSHarness(t)
	h.adguardEnabled()

	if err := h.vpn.enableVpnDNSForwarding(); err != nil {
		t.Fatalf("enable VPN DNS: %v", err)
	}
	if !slices.Equal(h.state.servers, []string{"10.8.0.1"}) || h.state.noresolv != "1" {
		t.Fatalf("the VPN must own dnsmasq while it is on top, got %v noresolv=%q",
			h.state.servers, h.state.noresolv)
	}
	st := h.readStack(t)
	if len(st.Layers) != 2 || st.Layers[0].Name != dnsLayerAdGuard ||
		st.Layers[1].Name != dnsLayerVPN {
		t.Fatalf("expected [adguard, vpn] on the stack, got %+v", st.Layers)
	}
	if len(st.Servers) != 1 || st.Servers[0] != "1.1.1.1" {
		t.Fatalf("the base must be the pre-any-layer list, got %v", st.Servers)
	}

	// AdGuard off: the VPN is still stacked on top, so dnsmasq is left alone.
	if err := h.adguard.SetDNS(false); err != nil {
		t.Fatalf("disable AdGuard DNS: %v", err)
	}
	if !slices.Equal(h.state.servers, []string{"10.8.0.1"}) || h.state.noresolv != "1" {
		t.Fatalf("the VPN layer must keep owning dnsmasq, got %v noresolv=%q",
			h.state.servers, h.state.noresolv)
	}
	if st := h.readStack(t); len(st.Layers) != 1 || st.Layers[0].Name != dnsLayerVPN {
		t.Fatalf("only the adguard entry must be removed, got %+v", st.Layers)
	}

	// VPN off last: the pre-any-layer state comes back and the record is dropped.
	h.vpn.disableVpnDNSForwarding()
	if !slices.Equal(h.state.servers, []string{"1.1.1.1"}) || h.state.noresolv != "0" {
		t.Fatalf("the last layer to disable must restore the base, got %v noresolv=%q",
			h.state.servers, h.state.noresolv)
	}
	if _, err := os.Stat(h.stackPath); !os.IsNotExist(err) {
		t.Errorf("the last layer to disable must drop the record, stat err = %v", err)
	}
}

// The same scenario with the order reversed: the VPN is on the bottom and
// AdGuard is stacked over it. Turning AdGuard off must restore the VPN's
// resolvers, not the pre-any-layer list.
func TestDNSLayerStack_VPNUnderAdGuardRestoresInOrder(t *testing.T) {
	h := newSharedDNSHarness(t)
	if err := h.vpn.enableVpnDNSForwarding(); err != nil {
		t.Fatalf("enable VPN DNS: %v", err)
	}
	h.adguardEnabled()

	if !slices.Equal(h.state.servers, []string{"127.0.0.1#5353"}) {
		t.Fatalf("AdGuard must own dnsmasq while it is on top, got %v", h.state.servers)
	}
	st := h.readStack(t)
	if len(st.Layers) != 2 || st.Layers[0].Name != dnsLayerVPN ||
		st.Layers[1].Name != dnsLayerAdGuard {
		t.Fatalf("expected [vpn, adguard] on the stack, got %+v", st.Layers)
	}

	// AdGuard off: the VPN's resolvers come back, not the pre-any-layer list.
	if err := h.adguard.SetDNS(false); err != nil {
		t.Fatalf("disable AdGuard DNS: %v", err)
	}
	if !slices.Equal(h.state.servers, []string{"10.8.0.1"}) || h.state.noresolv != "1" {
		t.Fatalf("the VPN layer below must be restored, got %v noresolv=%q",
			h.state.servers, h.state.noresolv)
	}
	if st := h.readStack(t); len(st.Layers) != 1 || st.Layers[0].Name != dnsLayerVPN {
		t.Fatalf("only the adguard entry must be removed, got %+v", st.Layers)
	}

	h.vpn.disableVpnDNSForwarding()
	if !slices.Equal(h.state.servers, []string{"1.1.1.1"}) || h.state.noresolv != "0" {
		t.Fatalf("the last layer to disable must restore the base, got %v noresolv=%q",
			h.state.servers, h.state.noresolv)
	}
	if _, err := os.Stat(h.stackPath); !os.IsNotExist(err) {
		t.Errorf("the last layer to disable must drop the record, stat err = %v", err)
	}
}

func TestSetConfig_RejectsInvalidYAML(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.fileContents[adguardYAMLPathOpt] = "bind_host: 0.0.0.0\nbind_port: 3000\n"
	svc := NewAdGuardServiceWithChecker(mock)

	// Unterminated flow mapping: parses as neither a document nor a mapping.
	err := svc.SetConfig("dns:\n  upstream_dns: [ \"1.1.1.1\"\n")
	if err == nil {
		t.Fatal("expected SetConfig to reject unparseable YAML")
	}
	if got := mock.fileContents[adguardYAMLPathOpt]; got != "bind_host: 0.0.0.0\nbind_port: 3000\n" {
		t.Errorf("the live config must not be touched when the body is invalid, got %q", got)
	}
	if _, ok := mock.commands[adguardInitd+" restart"]; ok {
		t.Error("AdGuard must not be restarted for an invalid body")
	}
}

func TestSetConfig_RejectsEmptyBody(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.fileContents[adguardYAMLPathOpt] = "bind_port: 3000\n"
	svc := NewAdGuardServiceWithChecker(mock)

	if err := svc.SetConfig("   \n"); err == nil {
		t.Fatal("expected SetConfig to reject an empty document")
	}
	if got := mock.fileContents[adguardYAMLPathOpt]; got != "bind_port: 3000\n" {
		t.Errorf("the live config must not be replaced by an empty document, got %q", got)
	}
}

// A config that AdGuard cannot start with must be rolled back: dnsmasq is
// already forwarding every LAN query to the listener.
func TestSetConfig_RestoresBackupWhenListenerNeverComesUp(t *testing.T) {
	prev := adguardConfigVerifyTimeout
	adguardConfigVerifyTimeout = 200 * time.Millisecond
	t.Cleanup(func() { adguardConfigVerifyTimeout = prev })

	const original = "bind_host: 0.0.0.0\nbind_port: 3000\ndns:\n  port: 5353\n"
	mock := newMockAdGuardChecker()
	mock.fileContents[adguardYAMLPathOpt] = original
	mock.commands[adguardInitd+" restart"] = struct {
		output string
		err    error
	}{"", nil}
	// Listener never comes back.
	mock.tcpProbes["127.0.0.1:5353"] = false
	svc := NewAdGuardServiceWithChecker(mock)

	if err := svc.SetConfig("bind_host: 0.0.0.0\nbind_port: 9999\n"); err == nil {
		t.Fatal("expected SetConfig to fail when the DNS listener never returns")
	}
	if got := mock.fileContents[adguardYAMLPathOpt]; got != original {
		t.Errorf("the previous config must be restored, got %q", got)
	}
	backups := 0
	for path := range mock.fileContents {
		if strings.HasSuffix(path, ".bak") {
			backups++
		}
	}
	if backups != 1 {
		t.Errorf("expected exactly one timestamped backup, got %d", backups)
	}
}

// The listener coming up late (a slow AdGuard start) must NOT trigger a rollback.
func TestSetConfig_SucceedsWhenListenerComesUpLate(t *testing.T) {
	prev := adguardConfigVerifyTimeout
	adguardConfigVerifyTimeout = 2 * time.Second
	prevPoll := adguardConfigVerifyPollDuration
	adguardConfigVerifyPollDuration = 10 * time.Millisecond
	t.Cleanup(func() {
		adguardConfigVerifyTimeout = prev
		adguardConfigVerifyPollDuration = prevPoll
	})

	mock := newMockAdGuardChecker()
	mock.fileContents[adguardYAMLPathOpt] = "bind_port: 3000\ndns:\n  port: 5353\n"
	mock.commands[adguardInitd+" restart"] = struct {
		output string
		err    error
	}{"", nil}
	// Down on the first probe (the pre-flight read), up on the second.
	mock.tcpProbeSeq["127.0.0.1:5353"] = []bool{false, true}
	svc := NewAdGuardServiceWithChecker(mock)

	if err := svc.SetConfig("bind_host: 0.0.0.0\nbind_port: 3000\ndns:\n  port: 5353\n"); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}
	if got := mock.fileContents[adguardYAMLPathOpt]; !strings.Contains(got, "bind_host") {
		t.Errorf("the new config must be kept, got %q", got)
	}
}

// refreshEndpointsFromYAML is called from five different entry points and
// rewrites three fields; without a mutex two concurrent requests could publish
// a base URL from one file with a port from another. Run with -race.
func TestAdGuardService_ConcurrentAccessIsRaceFree(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.fileContents[adguardYAMLPathOpt] = "bind_host: 0.0.0.0\nbind_port: 3000\ndns:\n  port: 5353\n"
	mock.httpGets["http://127.0.0.1:3000/control/status"] = struct {
		body []byte
		err  error
	}{body: []byte(`{"protection_enabled":true,"running":true}`)}
	mock.httpGets["http://127.0.0.1:3000/control/stats"] = struct {
		body []byte
		err  error
	}{body: []byte(`{"num_dns_queries":1}`)}
	svc := NewAdGuardServiceWithChecker(mock)

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				svc.refreshEndpointsFromYAML()
				_ = svc.apiBase()
				_, _ = svc.GetStatus()
				_ = svc.getDNSPort()
			}
		}()
	}
	wg.Wait()
}

// A successful change must not leave a backup behind: one file per edit would
// accumulate on the overlayfs NAND, and each is a copy of a bcrypt-holding file.
func TestSetConfig_RemovesBackupOnSuccess(t *testing.T) {
	mock := newMockAdGuardChecker()
	mock.fileContents[adguardYAMLPathOpt] = "bind_port: 3000\ndns:\n  port: 5353\n"
	mock.commands[adguardInitd+" restart"] = struct {
		output string
		err    error
	}{"", nil}
	mock.tcpProbes["127.0.0.1:5353"] = true
	svc := NewAdGuardServiceWithChecker(mock)

	if err := svc.SetConfig("bind_host: 0.0.0.0\nbind_port: 3000\ndns:\n  port: 5353\n"); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}
	for path := range mock.fileContents {
		if strings.HasSuffix(path, ".bak") {
			t.Errorf("the backup must be removed once the change is verified, found %s", path)
		}
	}
}
