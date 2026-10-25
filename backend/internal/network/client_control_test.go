package network

import (
	"fmt"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestBlockClient(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: &platform.MockCommandRunner{}})

	err := svc.BlockClient("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	blocked, err := svc.GetBlockedClients()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocked) != 1 {
		t.Fatalf("expected 1 blocked client, got %d", len(blocked))
	}
	if blocked[0] != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("expected blocked MAC 'AA:BB:CC:DD:EE:FF', got %q", blocked[0])
	}
}

func TestUnblockClient(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: &platform.MockCommandRunner{}})

	_ = svc.BlockClient("AA:BB:CC:DD:EE:FF")

	err := svc.UnblockClient("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	blocked, err := svc.GetBlockedClients()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocked) != 0 {
		t.Errorf("expected 0 blocked clients after unblock, got %d", len(blocked))
	}
}

func TestGetBlockedClients_Empty(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub})

	blocked, err := svc.GetBlockedClients()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocked) != 0 {
		t.Errorf("expected 0 blocked clients, got %d", len(blocked))
	}
}

func TestBlockClient_AlreadyBlocked(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: &platform.MockCommandRunner{}})

	_ = svc.BlockClient("AA:BB:CC:DD:EE:FF")
	err := svc.BlockClient("AA:BB:CC:DD:EE:FF")
	if err == nil {
		t.Error("expected error blocking already-blocked client")
	}
}

func TestUnblockClient_NotBlocked(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: &platform.MockCommandRunner{}})

	err := svc.UnblockClient("AA:BB:CC:DD:EE:FF")
	if err == nil {
		t.Error("expected error unblocking non-blocked client")
	}
}

func TestKickClient_SucceedsWhenAPInterfaceAccepts(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	iwDev := "phy0\n\tInterface phy0-ap0\n\t\ttype AP\n\tInterface phy0-sta0\n\t\ttype managed\n"
	runner := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "iw" {
			return []byte(iwDev), nil
		}
		return nil, nil // hostapd_cli disassociate succeeds
	}}
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: runner})

	if err := svc.KickClient("AA:BB:CC:DD:EE:FF"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// KickClient must report failure when nothing was actually disassociated,
// instead of returning nil and letting the UI claim success.
func TestKickClient_ErrorsWhenNoAPInterfaceFound(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: &platform.MockCommandRunner{Output: []byte("phy0\n")}})

	if err := svc.KickClient("AA:BB:CC:DD:EE:FF"); err == nil {
		t.Error("expected an error when no AP interface exists")
	}
}

func TestKickClient_ErrorsWhenAPInterfaceRejects(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	iwDev := "phy0\n\tInterface phy0-ap0\n\t\ttype AP\n"
	runner := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if name == "iw" {
			return []byte(iwDev), nil
		}
		return nil, fmt.Errorf("disassociate failed")
	}}
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: runner})

	if err := svc.KickClient("AA:BB:CC:DD:EE:FF"); err == nil {
		t.Error("expected an error when every AP interface rejects the disassociate")
	}
}

func TestBlockClient_CaseInsensitive(t *testing.T) {
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	svc := New(Deps{UCI: u, Ubus: ub, Cmd: &platform.MockCommandRunner{}})

	err := svc.BlockClient("aa:bb:cc:dd:ee:ff")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	blocked, _ := svc.GetBlockedClients()
	if len(blocked) != 1 {
		t.Fatalf("expected 1 blocked client, got %d", len(blocked))
	}
	if blocked[0] != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("expected MAC stored as uppercase, got %q", blocked[0])
	}
}

func TestGetBlockedClients_SurfacesGetSectionsError(t *testing.T) {
	svc := New(Deps{UCI: &uci.FailingSectionsUCI{MockUCI: uci.NewMockUCI()}, Ubus: ubus.NewMockUbus()})

	blocked, err := svc.GetBlockedClients()
	if err == nil {
		t.Fatal("expected an error instead of an empty blocked list when the UCI read fails")
	}
	if blocked != nil {
		t.Errorf("expected no list on error, got %v", blocked)
	}
}

func TestKickClient_NoInterfaceAcceptedReturnsError(t *testing.T) {
	calls := 0
	runner := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		calls++
		return nil, fmt.Errorf("no such interface")
	}}
	svc := New(Deps{UCI: uci.NewMockUCI(), Ubus: ubus.NewMockUbus(), Cmd: runner})

	if err := svc.KickClient("AA:BB:CC:DD:EE:FF"); err == nil {
		t.Fatal("expected an error when no interface accepted the disassociate")
	}
	if calls == 0 {
		t.Error("expected KickClient to try the discovered AP interfaces")
	}
}

func TestKickClient_SuccessWhenOneInterfaceAccepts(t *testing.T) {
	runner := &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		return []byte("Interface phy0-ap0\ntype AP\n"), nil
	}}
	svc := New(Deps{UCI: uci.NewMockUCI(), Ubus: ubus.NewMockUbus(), Cmd: runner})

	if err := svc.KickClient("AA:BB:CC:DD:EE:FF"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBlockClientRollsBackWhenFirewallReloadFails(t *testing.T) {
	u := uci.NewMockUCI()
	svc := New(Deps{UCI: u, Ubus: ubus.NewMockUbus(), Cmd: &platform.FuncCommandRunner{
		RunFunc: func(_ string, _ ...string) ([]byte, error) {
			return nil, fmt.Errorf("firewall reload failed")
		},
	}})

	if err := svc.BlockClient("AA:BB:CC:DD:EE:FF"); err == nil {
		t.Fatal("expected the block to be reported as failed")
	}
	blocked, err := svc.GetBlockedClients()
	if err != nil {
		t.Fatalf("GetBlockedClients: %v", err)
	}
	if len(blocked) != 0 {
		t.Errorf("a block whose reload failed must be rolled back, got %v", blocked)
	}
}

func TestUnblockClientRestoresRuleWhenFirewallReloadFails(t *testing.T) {
	u := uci.NewMockUCI()
	failing := &platform.FuncCommandRunner{
		RunFunc: func(_ string, _ ...string) ([]byte, error) {
			return nil, fmt.Errorf("firewall reload failed")
		},
	}
	svc := New(Deps{UCI: u, Ubus: ubus.NewMockUbus(), Cmd: failing})

	// Create the rule directly: the happy path cannot install one while the
	// reload is broken.
	section := "block_" + normalizeMACForSection("AA:BB:CC:DD:EE:FF")
	if err := u.AddSection("firewall", section, "rule"); err != nil {
		t.Fatalf("AddSection: %v", err)
	}
	for option, value := range map[string]string{
		"name": "Block-AA:BB:CC:DD:EE:FF", "src": "lan",
		"src_mac": "AA:BB:CC:DD:EE:FF", "target": "DROP",
	} {
		if err := u.Set("firewall", section, option, value); err != nil {
			t.Fatalf("Set: %v", err)
		}
	}

	if err := svc.UnblockClient("AA:BB:CC:DD:EE:FF"); err == nil {
		t.Fatal("expected the unblock to be reported as failed")
	}
	blocked, err := svc.GetBlockedClients()
	if err != nil {
		t.Fatalf("GetBlockedClients: %v", err)
	}
	if len(blocked) != 1 {
		t.Errorf("an unblock whose reload failed must put the rule back, got %v", blocked)
	}
}
