package uplink

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestSplitWireGuardDNSOption(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want []string
	}{
		{"198.18.0.1,198.18.0.2", []string{"198.18.0.1", "198.18.0.2"}},
		{"10.0.0.1 10.0.0.2", []string{"10.0.0.1", "10.0.0.2"}},
		{"  1.1.1.1 , 8.8.8.8 ", []string{"1.1.1.1", "8.8.8.8"}},
		{"", nil},
	}
	for _, tt := range tests {
		got := splitWireGuardDNSOption(tt.in)
		if len(got) != len(tt.want) {
			t.Errorf("splitWireGuardDNSOption(%q) = %v; want %v", tt.in, got, tt.want)
			continue
		}
		for i := range tt.want {
			if got[i] != tt.want[i] {
				t.Errorf("splitWireGuardDNSOption(%q)[%d] = %q; want %q", tt.in, i, got[i], tt.want[i])
			}
		}
	}
}

// Enabling twice must not overwrite the recorded base with the VPN's own
// resolvers. The dashboard toggle and the VPN page are independent entry points
// with no idempotency guard, so a second enable used to destroy the pre-VPN
// state and the later disable restored noresolv=1 pointing at 10.8.0.1.
func TestEnableVpnDNSForwarding_SecondEnableKeepsFirstSnapshot(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0", "dns", "10.8.0.1")
	cmd, state := newDnsmasqRunner([]string{"127.0.0.1#5353"}, true)
	svc, _ := newSnapshotScopedVpnService(t, u, cmd)

	if err := svc.enableVpnDNSForwarding(); err != nil {
		t.Fatalf("first enable: %v", err)
	}
	if !slices.Equal(state.servers, []string{"10.8.0.1"}) || state.noresolv != "1" {
		t.Fatalf("first enable must point dnsmasq at the tunnel,"+
			" got %v noresolv=%q", state.servers, state.noresolv)
	}
	st := readVpnLayerStack(t, svc)
	if !slices.Equal(st.Servers, []string{"127.0.0.1#5353"}) {
		t.Fatalf("the record must hold the pre-VPN list as the base, got %v", st.Servers)
	}
	if len(st.Layers) != 1 || st.Layers[0].Name != dnsLayerVPN {
		t.Fatalf("expected exactly one stacked layer (vpn), got %+v", st.Layers)
	}

	// Second enable, now that dnsmasq already points at the tunnel.
	if err := svc.enableVpnDNSForwarding(); err != nil {
		t.Fatalf("second enable: %v", err)
	}
	if again := readVpnLayerStack(t, svc); !slices.Equal(again.Servers, st.Servers) ||
		len(again.Layers) != 1 {
		t.Fatalf("a second enable must not re-read dnsmasq as the base:\nbefore %+v\nafter %+v",
			st, again)
	}

	// The disable still restores the pre-VPN list, not the tunnel's.
	svc.disableVpnDNSForwarding()
	if !slices.Equal(state.servers, []string{"127.0.0.1#5353"}) {
		t.Fatalf("disable must restore the pre-VPN list, got %v", state.servers)
	}
	if state.noresolv != "0" {
		t.Fatalf("disable must restore the pre-VPN noresolv, got %q", state.noresolv)
	}
	if _, err := os.Stat(svc.dnsStackPath); !os.IsNotExist(err) {
		t.Errorf("the last layer to disable must drop the record, stat err = %v", err)
	}
}

// A record that cannot be written means there is nothing to restore, so the
// forwarding must be aborted before dnsmasq is touched at all.
func TestEnableVpnDNSForwarding_SnapshotWriteFailureLeavesDnsmasqAlone(t *testing.T) {
	u := uci.NewMockUCI()
	_ = u.Set("network", "wg0", "dns", "10.8.0.1")
	cmd, state := newDnsmasqRunner([]string{"127.0.0.1#5353"}, true)
	svc, _ := newSnapshotScopedVpnService(t, u, cmd)
	// A regular file cannot be a parent directory, so the record write fails.
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	svc.dnsStackPath = filepath.Join(blocker, "dnsmasq-layers.json")

	if err := svc.enableVpnDNSForwarding(); err == nil {
		t.Fatal("expected the forwarding to abort when the record cannot be written")
	}
	if !slices.Equal(state.servers, []string{"127.0.0.1#5353"}) {
		t.Errorf("dnsmasq must be untouched when the record failed, got %v", state.servers)
	}
	if state.noresolv != "" {
		t.Errorf("noresolv must not be set when the record failed, got %q", state.noresolv)
	}
}
