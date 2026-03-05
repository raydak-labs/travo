package services

import (
	"fmt"
	"testing"
)

// mockAdGuardChecker is a test double for AdGuardChecker.
type mockAdGuardChecker struct {
	files    map[string]bool
	commands map[string]struct {
		output string
		err    error
	}
	httpGets map[string]struct {
		body []byte
		err  error
	}
}

func newMockAdGuardChecker() *mockAdGuardChecker {
	return &mockAdGuardChecker{
		files: make(map[string]bool),
		commands: make(map[string]struct {
			output string
			err    error
		}),
		httpGets: make(map[string]struct {
			body []byte
			err  error
		}),
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
