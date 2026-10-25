package wifi

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/platform"
)

func TestSetMACAddress(t *testing.T) {
	svc, u := newMACUnitTestService()

	// Input is canonicalized to lowercase colon notation.
	_, err := svc.SetMACAddress("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify MAC was set
	opts, err := u.GetAll("wireless", "sta0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts["macaddr"] != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("expected macaddr 'aa:bb:cc:dd:ee:ff', got '%s'", opts["macaddr"])
	}
}

func TestSetMACAddress_Reset(t *testing.T) {
	svc, u := newMACUnitTestService()

	// Set then reset
	_, _ = svc.SetMACAddress("AA:BB:CC:DD:EE:FF")
	_, err := svc.SetMACAddress("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	opts, err := u.GetAll("wireless", "sta0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts["macaddr"] != "" {
		t.Errorf("expected empty macaddr, got '%s'", opts["macaddr"])
	}
}

func TestGetMACAddresses(t *testing.T) {
	svc, _ := newTestWifiService()

	configs, err := svc.GetMACAddresses()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) == 0 {
		t.Fatal("expected at least one MAC config")
	}
	if configs[0].Interface != "sta" {
		t.Errorf("expected interface 'sta', got '%s'", configs[0].Interface)
	}
}

func TestRandomizeMAC(t *testing.T) {
	svc, u := newMACUnitTestService()

	mac, _, err := svc.RandomizeMAC()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify MAC format (XX:XX:XX:XX:XX:XX)
	if len(mac) != 17 {
		t.Fatalf("expected 17 char MAC, got %d: %s", len(mac), mac)
	}

	// Parse first octet to check locally-administered + unicast
	var firstOctet int
	if _, err := fmt.Sscanf(mac[:2], "%x", &firstOctet); err != nil {
		t.Fatalf("failed to parse first octet: %v", err)
	}
	if firstOctet&0x02 == 0 {
		t.Error("expected locally-administered bit set (bit 1 of first octet)")
	}
	if firstOctet&0x01 != 0 {
		t.Error("expected unicast bit cleared (bit 0 of first octet)")
	}

	// Verify MAC was applied in UCI
	opts, err := u.GetAll("wireless", "sta0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts["macaddr"] != mac {
		t.Errorf("expected macaddr '%s', got '%s'", mac, opts["macaddr"])
	}
}

func TestRandomizeMAC_UniquePerCall(t *testing.T) {
	svc, _ := newMACUnitTestService()

	mac1, _, err := svc.RandomizeMAC()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mac2, _, err := svc.RandomizeMAC()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Extremely unlikely to be the same with 46 bits of randomness
	if mac1 == mac2 {
		t.Errorf("expected different MACs, both were %s", mac1)
	}
}

func TestSetMACAddress_RejectsInvalidMAC(t *testing.T) {
	svc, u := newTestWifiService()
	for _, mac := range []string{"not-a-mac", "AA:BB:CC:DD:EE", "ZZ:BB:CC:DD:EE:FF", "AA:BB:CC:DD:EE:FF:00", "AABBCCDDEEFF"} {
		if _, err := svc.SetMACAddress(mac); err == nil {
			t.Errorf("expected %q to be rejected as a MAC address", mac)
		}
		if opts, _ := u.GetAll("wireless", "sta0"); opts["macaddr"] != "" {
			t.Errorf("invalid MAC %q must not be written to wireless.sta0.macaddr", mac)
		}
	}
}

func TestSetMACAddress_AcceptsValidVariants(t *testing.T) {
	svc, u := newMACUnitTestService()
	// Accepted spellings are canonicalized to lowercase colon notation, which
	// is what netifd and mac80211.sh expect.
	for mac, want := range map[string]string{
		"AA:BB:CC:DD:EE:FF":     "aa:bb:cc:dd:ee:ff",
		"aa:bb:cc:dd:ee:ff":     "aa:bb:cc:dd:ee:ff",
		"AA-BB-CC-DD-EE-FF":     "aa:bb:cc:dd:ee:ff",
		"  AA:BB:CC:DD:EE:FF  ": "aa:bb:cc:dd:ee:ff",
	} {
		if _, err := svc.SetMACAddress(mac); err != nil {
			t.Errorf("expected %q to be accepted: %v", mac, err)
		}
		opts, _ := u.GetAll("wireless", "sta0")
		if opts["macaddr"] != want {
			t.Errorf("input %q: expected macaddr %q, got %q", mac, want, opts["macaddr"])
		}
	}
}

func TestSetMACAddress_GuardRemovedAndLinkTouchedAfterApply(t *testing.T) {
	guardDir := t.TempDir()
	var cmds []string
	svc, u := newMACTestService(t, guardDir, &fakeWirelessApplier{startToken: "t1"}, &cmds)

	guard := filepath.Join(guardDir, "mac-in-progress")
	applied, err := svc.SetMACAddress("AA:BB:CC:DD:EE:FF")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if applied == nil {
		t.Fatal("expected an apply result")
	}
	if _, statErr := os.Stat(guard); !os.IsNotExist(statErr) {
		t.Errorf("expected the crash guard %s to be removed after success", guard)
	}
	if opts, _ := u.GetAll("wireless", "sta0"); opts["macaddr"] != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("expected macaddr to be committed, got %v", opts)
	}
	if len(cmds) == 0 {
		t.Fatal("expected the MAC to be applied to the live link")
	}
	// The link must only be touched once the config is staged and verified, so
	// it can be the first and only thing that runs.
	for i, c := range cmds {
		if !strings.HasPrefix(c, "ip link set") {
			t.Errorf("command %d (%q) is not link manipulation: %v", i, c, cmds)
		}
	}
	if !strings.Contains(strings.Join(cmds, " "), "address aa:bb:cc:dd:ee:ff") {
		t.Errorf("expected the new MAC to be set on the link, commands: %v", cmds)
	}
}

// The `ip link set <if> address` step is the only one that can leave the router
// worse off: the interface was just taken DOWN, and if the address is rejected
// the function returned without bringing it back up and the caller swallowed the
// error — so the STA silently disappeared behind a 200 response, taking the
// operator's upstream link with it.
func TestSetMACAddress_LinkFailureIsReturnedAndLinkBroughtBackUp(t *testing.T) {
	guardDir := t.TempDir()
	var cmds []string
	svc, _ := newMACTestService(t, guardDir, &fakeWirelessApplier{startToken: "t1"}, &cmds)
	svc.cmd = &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		cmds = append(cmds, strings.Join(append([]string{name}, args...), " "))
		// The address change is what a busy driver rejects.
		if slices.Contains(args, "address") {
			return nil, errors.New("RTNETLINK answers: Operation not supported")
		}
		return nil, nil
	}}

	apply, err := svc.SetMACAddress("AA:BB:CC:DD:EE:FF")
	if err == nil {
		t.Fatal("a failed `ip link set address` must surface as an error, not a success")
	}
	if apply != nil {
		t.Errorf("no apply result may be reported when the live link failed: %+v", apply)
	}
	joined := strings.Join(cmds, " | ")
	if !strings.Contains(joined, "ip link set phy0-sta0 down") {
		t.Fatalf("the link was never taken down, so this test proves nothing: %v", cmds)
	}
	// After the failed address change the link must be brought back up.
	if strings.Count(joined, "ip link set phy0-sta0 up") != 1 {
		t.Errorf("the interface was left down after a failed address change: %v", cmds)
	}
}

// The guard is the only durable record that the MAC sequence did not finish
// (ADR 0003 §1.3: remove it only after the operation completes end to end). It
// was cleared unconditionally as soon as the apply was staged, before the live
// link had been touched at all.
func TestSetMACAddress_GuardSurvivesAFailedLinkChange(t *testing.T) {
	guardDir := t.TempDir()
	var cmds []string
	svc, _ := newMACTestService(t, guardDir, &fakeWirelessApplier{startToken: "t1"}, &cmds)
	svc.cmd = &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		if slices.Contains(args, "address") {
			return nil, errors.New("RTNETLINK answers: Operation not supported")
		}
		return nil, nil
	}}

	if _, err := svc.SetMACAddress("AA:BB:CC:DD:EE:FF"); err == nil {
		t.Fatal("expected the link failure to surface")
	}
	if _, err := os.Stat(filepath.Join(guardDir, "mac-in-progress")); err != nil {
		t.Errorf("the crash guard must survive an incomplete MAC change: %v", err)
	}
}

// A `ip link down` that fails means nothing was changed, but the sequence still
// did not complete; the guard must stay so the operator can see the device is in
// an unresolved state.
func TestSetMACAddress_GuardSurvivesAFailedLinkDown(t *testing.T) {
	guardDir := t.TempDir()
	var cmds []string
	svc, _ := newMACTestService(t, guardDir, &fakeWirelessApplier{startToken: "t1"}, &cmds)
	svc.cmd = &platform.MockCommandRunner{RunFunc: func(name string, args ...string) ([]byte, error) {
		cmds = append(cmds, strings.Join(append([]string{name}, args...), " "))
		if slices.Contains(args, "down") {
			return nil, errors.New("ip: cannot find device phy0-sta0")
		}
		return nil, nil
	}}

	if _, err := svc.SetMACAddress("AA:BB:CC:DD:EE:FF"); err == nil {
		t.Fatal("expected the link failure to surface")
	}
	if _, err := os.Stat(filepath.Join(guardDir, "mac-in-progress")); err != nil {
		t.Errorf("the crash guard must survive an incomplete MAC change: %v", err)
	}
	if slices.ContainsFunc(cmds, func(c string) bool { return strings.Contains(c, "address") }) {
		t.Errorf("the address must not be set after the interface could not be taken down: %v", cmds)
	}
}

func TestSetMACAddress_ApplyFailureKeepsGuardAndLeavesLinkAlone(t *testing.T) {
	guardDir := t.TempDir()
	var cmds []string
	svc, _ := newMACTestService(t, guardDir, &fakeWirelessApplier{startErr: errors.New("apply failed")}, &cmds)

	if _, err := svc.SetMACAddress("AA:BB:CC:DD:EE:FF"); err == nil {
		t.Fatal("expected the apply failure to surface")
	}
	if len(cmds) != 0 {
		t.Errorf("the live link must not be touched when the apply failed, commands: %v", cmds)
	}
	guard := filepath.Join(guardDir, "mac-in-progress")
	if _, err := os.Stat(guard); err != nil {
		t.Errorf("expected crash guard %s to remain so recovery can act on it: %v", guard, err)
	}
}

func TestSetMACAddress_GuardWrittenBeforeUCIWrites(t *testing.T) {
	guardDir := t.TempDir()
	var cmds []string
	// An applier that inspects the guard while the apply is being staged.
	guardPath := filepath.Join(guardDir, "mac-in-progress")
	applier := &guardCheckingApplier{guardPath: guardPath, guardMissing: func() bool {
		_, err := os.Stat(guardPath)
		return os.IsNotExist(err)
	}}
	svc, _ := newMACTestService(t, guardDir, applier, &cmds)

	if _, err := svc.SetMACAddress("AA:BB:CC:DD:EE:FF"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if applier.guardWasMissing {
		t.Error("the crash guard must be written before the wireless config is applied")
	}
}
