package uplink

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

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
	svc := NewAdGuardService(mock)

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
	svc := NewAdGuardService(mock)

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
	svc := NewAdGuardService(mock)

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
	svc := NewAdGuardService(mock)

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
	svc := NewAdGuardService(mock)

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
	svc := NewAdGuardService(mock)

	err := svc.SetDNS(true)
	if err == nil {
		t.Error("expected error when add_list fails")
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
	svc := NewAdGuardService(mock)
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
	svc := NewAdGuardService(mock)
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
	svc := NewAdGuardService(mock)

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
