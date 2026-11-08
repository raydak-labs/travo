package platform

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/auth"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
)

func TestExtractSessionID(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		want string
	}{
		{"top-level", map[string]any{"ubus_rpc_session": "abc123"}, "abc123"},
		{"empty", map[string]any{}, ""},
		{"result-array", map[string]any{
			"result": []any{0, map[string]any{"ubus_rpc_session": "sid456"}},
		}, "sid456"},
		{"result-array-no-second", map[string]any{
			"result": []any{0},
		}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ubus.ExtractSessionID(tt.m)
			if got != tt.want {
				t.Errorf("extractSessionID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNoopUCIApplyConfirm_ApplyAndConfirm(t *testing.T) {
	var n NoopUCIApplyConfirm
	if err := n.ApplyAndConfirm([]string{"wireless", "network"}); err != nil {
		t.Errorf("NoopUCIApplyConfirm.ApplyAndConfirm() error = %v", err)
	}
}

func TestRealUCIApplyConfirm_UsesPasswordFromHolder(t *testing.T) {
	mub := ubus.NewMockUbus()
	pw := auth.NewRootPassword()
	pw.Set("my-secret-password")

	applier := NewRealUCIApplyConfirm(mub, pw)

	// Verify the applier is wired to the password holder.
	// We can't call sessionLogin directly (unexported), but we can verify
	// that the constructor accepts and stores the password holder.
	if applier.password.Get() != "my-secret-password" {
		t.Errorf("expected applier to have 'my-secret-password', got %q", applier.password.Get())
	}
}

func TestRealUCIApplyConfirm_EmptyPasswordWhenNotSet(t *testing.T) {
	mub := ubus.NewMockUbus()
	pw := auth.NewRootPassword()

	applier := NewRealUCIApplyConfirm(mub, pw)

	if applier.password.Get() != "" {
		t.Errorf("expected empty password, got %q", applier.password.Get())
	}
}

func TestRootPassword_GetSet(t *testing.T) {
	pw := auth.NewRootPassword()

	if pw.Get() != "" {
		t.Error("expected empty password initially")
	}

	pw.Set("test123")
	if pw.Get() != "test123" {
		t.Errorf("expected 'test123', got %q", pw.Get())
	}

	pw.Set("newpass")
	if pw.Get() != "newpass" {
		t.Errorf("expected 'newpass', got %q", pw.Get())
	}
}

// newTestUCIApply builds a RealUCIApplyConfirm wired to temp dirs and a fake ubus.
func newTestUCIApply(t *testing.T, fu *FakeUbusApply) (*RealUCIApplyConfirm, string, string) {
	t.Helper()
	root := t.TempDir()
	etcDir := filepath.Join(root, "etc", "config")
	runDir := filepath.Join(root, "var", "run", "rpcd")
	if err := os.MkdirAll(etcDir, 0700); err != nil {
		t.Fatalf("mkdir etc config dir: %v", err)
	}
	pw := auth.NewRootPassword()
	pw.Set("secret")
	applier := NewRealUCIApplyConfirm(fu, pw)
	applier.EtcConfigDir = etcDir
	applier.RPCDRunDir = runDir
	return applier, etcDir, runDir
}

func TestRealUCIApplyConfirm_StartApplyEmptyListIsNoop(t *testing.T) {
	t.Parallel()

	fu := &FakeUbusApply{}
	applier, _, _ := newTestUCIApply(t, fu)

	sid, err := applier.StartApply(nil)
	if err != nil {
		t.Fatalf("StartApply(nil) error = %v, want nil", err)
	}
	if sid != "" {
		t.Errorf("StartApply(nil) session = %q, want empty", sid)
	}
	if len(fu.calls) != 0 {
		t.Errorf("expected no ubus calls for empty config list, got %d", len(fu.calls))
	}
}

func TestRealUCIApplyConfirm_ApplyAndConfirmEmptyListIsNoop(t *testing.T) {
	t.Parallel()

	fu := &FakeUbusApply{}
	applier, _, _ := newTestUCIApply(t, fu)

	if err := applier.ApplyAndConfirm([]string{}); err != nil {
		t.Fatalf("ApplyAndConfirm([]) error = %v, want nil", err)
	}
	if len(fu.calls) != 0 {
		t.Errorf("expected no ubus calls for empty config list, got %d", len(fu.calls))
	}
}

// The #220 regression. rpcd 2026.10.04 acts on a session's pending uci changes
// and ignores files dropped into /var/run/rpcd/uci-<session id>; with nothing
// pending, `uci apply` answers "No response". StartApply therefore has to create
// a pending change through the session itself, and FakeUbusApply refuses an
// apply without one, so staging files by hand can no longer pass this test.
func TestRealUCIApplyConfirm_StartApplyAppliesAPendingChangeMadeThroughTheSession(t *testing.T) {
	t.Parallel()

	fu := &FakeUbusApply{LoginSID: "sess-42"}
	applier, etcDir, runDir := newTestUCIApply(t, fu)

	for _, name := range []string{"mwan3", "network"} {
		if err := os.WriteFile(filepath.Join(etcDir, name), []byte("config x 'x'\n"), 0600); err != nil {
			t.Fatalf("write %s config: %v", name, err)
		}
	}

	sid, err := applier.StartApply([]string{"network", "mwan3"})
	if err != nil {
		t.Fatalf("StartApply error = %v", err)
	}
	if sid != "sess-42" {
		t.Fatalf("session = %q, want sess-42", sid)
	}
	if got := fu.PendingConfigs(sid); !slices.Equal(got, []string{"mwan3", "network"}) {
		t.Errorf("pending configs in the session = %v, want both configs", got)
	}
	// Nothing may be hand-staged into rpcd's run dir any more.
	if entries, _ := os.ReadDir(runDir); len(entries) != 0 {
		t.Errorf("StartApply wrote into %s: %v", runDir, entries)
	}

	applyCall := fu.find("uci", "apply")
	if applyCall == nil {
		t.Fatal("expected uci.apply call")
	}
	if applyCall.args["rollback"] != true {
		t.Errorf("rollback = %v, want true", applyCall.args["rollback"])
	}
	if applyCall.args["timeout"] != ApplyRollbackTimeout {
		t.Errorf("timeout = %v, want %d", applyCall.args["timeout"], ApplyRollbackTimeout)
	}
	if applyCall.args["ubus_rpc_session"] != "sess-42" {
		t.Errorf("ubus_rpc_session = %v, want sess-42", applyCall.args["ubus_rpc_session"])
	}
	if fu.find("uci", "confirm") != nil {
		t.Error("StartApply must not confirm before the caller verifies")
	}
}

// The marker must leave the real config unchanged: it is added and deleted in
// the same session, so the net delta rpcd commits is empty.
func TestRealUCIApplyConfirm_StartApplyMarkerIsAddedThenDeletedPerConfig(t *testing.T) {
	t.Parallel()

	fu := &FakeUbusApply{}
	applier, etcDir, _ := newTestUCIApply(t, fu)
	if err := os.WriteFile(filepath.Join(etcDir, "wireless"), []byte("config x 'x'\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := applier.StartApply([]string{"wireless"}); err != nil {
		t.Fatalf("StartApply error = %v", err)
	}
	add, del := fu.indexOf("uci", "add"), fu.indexOf("uci", "delete")
	apply := fu.indexOf("uci", "apply")
	if add < 0 || del <= add || apply <= del {
		t.Fatalf("want add < delete < apply, got add=%d delete=%d apply=%d: %+v", add, del, apply, fu.calls)
	}
	if fu.calls[add].args["name"] != fu.calls[del].args["section"] {
		t.Errorf("marker added as %v but deleted as %v", fu.calls[add].args["name"], fu.calls[del].args["section"])
	}
}

// A config that does not exist (mwan3 not installed) is skipped, as before. If
// every config is skipped there is nothing to apply, which is the "applier had
// nothing to do" empty-session answer, not an error.
func TestRealUCIApplyConfirm_StartApplySkipsMissingConfig(t *testing.T) {
	t.Parallel()

	fu := &FakeUbusApply{LoginSID: "sess-skip"}
	applier, etcDir, _ := newTestUCIApply(t, fu)
	if err := os.WriteFile(filepath.Join(etcDir, "network"), []byte("config x 'x'\n"), 0600); err != nil {
		t.Fatal(err)
	}

	sid, err := applier.StartApply([]string{"mwan3", "network"})
	if err != nil || sid != "sess-skip" {
		t.Fatalf("StartApply = (%q, %v), want the session with mwan3 skipped", sid, err)
	}
	if got := fu.PendingConfigs(sid); !slices.Equal(got, []string{"network"}) {
		t.Errorf("pending configs = %v, want only network", got)
	}

	fu2 := &FakeUbusApply{}
	applier2, _, _ := newTestUCIApply(t, fu2)
	sid, err = applier2.StartApply([]string{"mwan3"})
	if err != nil || sid != "" {
		t.Errorf("StartApply with nothing to apply = (%q, %v), want (\"\", nil)", sid, err)
	}
	if fu2.find("uci", "apply") != nil {
		t.Error("uci.apply must not run when no config was staged")
	}
}

// A failed apply must not leave the marker changes pending in a session nobody
// will confirm.
func TestRealUCIApplyConfirm_StartApplyRevertsTheSessionOnApplyFailure(t *testing.T) {
	t.Parallel()

	fu := &FakeUbusApply{LoginSID: "sess-fail", ApplyErr: errors.New("apply boom")}
	applier, etcDir, _ := newTestUCIApply(t, fu)
	if err := os.WriteFile(filepath.Join(etcDir, "mwan3"), []byte("config x 'x'\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := applier.StartApply([]string{"mwan3"}); err == nil {
		t.Fatal("expected StartApply to fail when uci.apply fails")
	}
	if got := fu.PendingConfigs("sess-fail"); len(got) != 0 {
		t.Errorf("failed apply left pending changes in the session: %v", got)
	}
}

func TestRealUCIApplyConfirm_StartApplyFailsWhenTheSessionRefusesTheMarker(t *testing.T) {
	t.Parallel()

	fu := &FakeUbusApply{AddErr: errors.New("permission denied")}
	applier, etcDir, _ := newTestUCIApply(t, fu)
	if err := os.WriteFile(filepath.Join(etcDir, "mwan3"), []byte("config x 'x'\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := applier.StartApply([]string{"mwan3"}); err == nil {
		t.Fatal("expected StartApply to fail when the session cannot stage a change")
	}
	if fu.find("uci", "apply") != nil {
		t.Error("uci.apply must not run when staging failed")
	}
}

// Config A is staged, then config B's marker cannot be deleted: nothing may stay
// pending in the session, or a later apply would commit a stray marker section.
func TestRealUCIApplyConfirm_StartApplyRevertsEveryStagedConfigWhenALaterOneFails(t *testing.T) {
	t.Parallel()

	fu := &FakeUbusApply{
		LoginSID:     "sess-half",
		DeleteErrFor: map[string]error{"network": errors.New("delete boom")},
	}
	applier, etcDir, _ := newTestUCIApply(t, fu)
	for _, name := range []string{"wireless", "network"} {
		if err := os.WriteFile(filepath.Join(etcDir, name), []byte("config x 'x'\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := applier.StartApply([]string{"wireless", "network"}); err == nil {
		t.Fatal("expected StartApply to fail when a marker cannot be deleted")
	}
	if fu.find("uci", "apply") != nil {
		t.Error("uci.apply must not run after a staging failure")
	}
	if got := fu.PendingConfigs("sess-half"); len(got) != 0 {
		t.Errorf("session still has pending changes after the failure: %v", got)
	}
}

func TestRealUCIApplyConfirm_StartApplyFailsWithoutSession(t *testing.T) {
	t.Parallel()

	fu := &FakeUbusApply{LoginErr: errors.New("login denied")}
	applier, _, _ := newTestUCIApply(t, fu)

	if _, err := applier.StartApply([]string{"network"}); err == nil {
		t.Fatal("expected StartApply to fail when session login fails")
	}
	if fu.find("uci", "apply") != nil {
		t.Error("uci.apply must not run without a session")
	}
}

func TestRealUCIApplyConfirm_Confirm(t *testing.T) {
	t.Parallel()

	fu := &FakeUbusApply{}
	applier, _, _ := newTestUCIApply(t, fu)

	if err := applier.Confirm(""); err == nil {
		t.Error("expected error confirming an empty session id")
	}
	if err := applier.Confirm("sess-1"); err != nil {
		t.Fatalf("Confirm error = %v", err)
	}
	call := fu.find("uci", "confirm")
	if call == nil {
		t.Fatal("expected uci.confirm call")
	}
	if call.args["ubus_rpc_session"] != "sess-1" {
		t.Errorf("ubus_rpc_session = %v, want sess-1", call.args["ubus_rpc_session"])
	}
}

func TestRealUCIApplyConfirm_ApplyAndConfirmRunsApplyThenConfirm(t *testing.T) {
	t.Parallel()

	fu := &FakeUbusApply{LoginSID: "sess-9"}
	applier, etcDir, _ := newTestUCIApply(t, fu)
	if err := os.WriteFile(filepath.Join(etcDir, "mwan3"), []byte("config mwan3 'x'\n"), 0600); err != nil {
		t.Fatalf("write mwan3 config: %v", err)
	}

	if err := applier.ApplyAndConfirm([]string{"mwan3"}); err != nil {
		t.Fatalf("ApplyAndConfirm error = %v", err)
	}
	applyIdx, confirmIdx := -1, -1
	for i, c := range fu.calls {
		if c.path == "uci" && c.method == "apply" && applyIdx < 0 {
			applyIdx = i
		}
		if c.path == "uci" && c.method == "confirm" && confirmIdx < 0 {
			confirmIdx = i
		}
	}
	if applyIdx < 0 || confirmIdx < 0 {
		t.Fatalf("expected apply then confirm, calls = %+v", fu.calls)
	}
	if applyIdx > confirmIdx {
		t.Errorf("uci.apply must precede uci.confirm (apply=%d confirm=%d)", applyIdx, confirmIdx)
	}
}

// With none of the configs present StartApply applies nothing and returns no
// session. ApplyAndConfirm used to read that as a failed confirm and roll back.
func TestRealUCIApplyConfirm_ApplyAndConfirmWithNoExistingConfigIsANoop(t *testing.T) {
	t.Parallel()

	fu := &FakeUbusApply{}
	applier, _, _ := newTestUCIApply(t, fu)

	if err := applier.ApplyAndConfirm([]string{"mwan3"}); err != nil {
		t.Fatalf("ApplyAndConfirm error = %v, want nil", err)
	}
	if fu.find("uci", "confirm") != nil || fu.find("uci", "reload_config") != nil {
		t.Errorf("nothing was applied, so nothing may be confirmed or rolled back: %+v", fu.calls)
	}
}

func TestRealUCIApplyConfirm_DefaultsToDevicePaths(t *testing.T) {
	t.Parallel()

	applier := NewRealUCIApplyConfirm(&FakeUbusApply{}, auth.NewRootPassword())
	if applier.RPCDRunDir != "/var/run/rpcd" {
		t.Errorf("rpcdRunDir = %q, want /var/run/rpcd", applier.RPCDRunDir)
	}
	if applier.EtcConfigDir != "/etc/config" {
		t.Errorf("etcConfigDir = %q, want /etc/config", applier.EtcConfigDir)
	}
}

const snapshotWireless = "config wifi-device 'radio0'\n\tenabled '1'\n"

// The hardware finding: Travo commits the UCI change BEFORE either rollback
// mechanism runs, so rpcd's window snapshots the already-changed config and
// `uci revert` is a no-op on a committed delta. A refused confirm therefore left
// the broken profile enabled. Only a file-level snapshot taken BEFORE the
// mutation commits can put the previous config back.
func TestRealUCIApplyConfirm_RollbackRestoresACommittedConfig(t *testing.T) {
	t.Parallel()

	fu := &FakeUbusApply{LoginSID: "sess-rb"}
	applier, etcDir, _ := newTestUCIApply(t, fu)
	cfg := filepath.Join(etcDir, "wireless")
	if err := os.WriteFile(cfg, []byte(snapshotWireless), 0600); err != nil {
		t.Fatalf("write wireless config: %v", err)
	}

	if err := applier.Snapshot([]string{"wireless"}); err != nil {
		t.Fatalf("Snapshot error = %v", err)
	}
	// The mutation commits: this is the state a refused confirm used to leave.
	committed := "config wifi-iface 'sta0'\n\tmode 'sta'\n\tdisabled '0'\n"
	if err := os.WriteFile(cfg, []byte(committed), 0600); err != nil {
		t.Fatalf("commit wireless config: %v", err)
	}
	sid, err := applier.StartApply([]string{"wireless"})
	if err != nil {
		t.Fatalf("StartApply error = %v", err)
	}

	if err := applier.Rollback(sid); err != nil {
		t.Fatalf("Rollback error = %v", err)
	}
	got, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read wireless config after rollback: %v", err)
	}
	if string(got) != snapshotWireless {
		t.Errorf("wireless config after rollback =\n%q\nwant the snapshot\n%q", got, snapshotWireless)
	}
}

// Confirm is the success path: the snapshot it discards must not be able to
// undo a LATER, unrelated change.
func TestRealUCIApplyConfirm_ConfirmDiscardsTheSnapshot(t *testing.T) {
	t.Parallel()

	fu := &FakeUbusApply{LoginSID: "sess-ok"}
	applier, etcDir, _ := newTestUCIApply(t, fu)
	cfg := filepath.Join(etcDir, "wireless")
	if err := os.WriteFile(cfg, []byte(snapshotWireless), 0600); err != nil {
		t.Fatalf("write wireless config: %v", err)
	}

	if err := applier.Snapshot([]string{"wireless"}); err != nil {
		t.Fatalf("Snapshot error = %v", err)
	}
	sid, err := applier.StartApply([]string{"wireless"})
	if err != nil {
		t.Fatalf("StartApply error = %v", err)
	}
	if err := applier.Confirm(sid); err != nil {
		t.Fatalf("Confirm error = %v", err)
	}
	// A later, unrelated edit that must survive a stale rollback attempt.
	later := "config wifi-iface 'guest'\n\tmode 'ap'\n"
	if err := os.WriteFile(cfg, []byte(later), 0600); err != nil {
		t.Fatalf("write later config: %v", err)
	}

	if err := applier.Rollback(sid); err == nil {
		t.Fatal("Rollback after Confirm must fail: the snapshot is gone, so there is nothing to restore")
	}
	got, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(got) != later {
		t.Errorf("config was overwritten by a stale rollback: %q", got)
	}
}

// A rollback that cannot find what to restore must not report success.
func TestRealUCIApplyConfirm_RollbackWithoutASnapshotFails(t *testing.T) {
	t.Parallel()

	fu := &FakeUbusApply{}
	applier, etcDir, _ := newTestUCIApply(t, fu)
	cfg := filepath.Join(etcDir, "wireless")
	if err := os.WriteFile(cfg, []byte("changed\n"), 0600); err != nil {
		t.Fatalf("write wireless config: %v", err)
	}

	if err := applier.Rollback("sess-never"); err == nil {
		t.Fatal("expected an error when no snapshot exists for the session")
	}
	got, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(got) != "changed\n" {
		t.Errorf("a failed rollback must not touch the config, got %q", got)
	}
	if fu.indexOf("uci", "reload_config") >= 0 || fu.indexOf("network", "reload") >= 0 {
		t.Error("a rollback that restored nothing must not ask the device to reload")
	}
}

// Restoring files is not enough: rpcd and netifd have to be told to re-read
// them. `network reload` is the bounded-recovery exception to the "no wifi up"
// rule (ADR 0003) and is the ONLY thing allowed to act on the restored config.
func TestRealUCIApplyConfirm_RollbackReloadsUciAndNetworkAndNeverRunsWifi(t *testing.T) {
	t.Parallel()

	fu := &FakeUbusApply{LoginSID: "sess-load"}
	applier, etcDir, _ := newTestUCIApply(t, fu)
	cfg := filepath.Join(etcDir, "wireless")
	if err := os.WriteFile(cfg, []byte(snapshotWireless), 0600); err != nil {
		t.Fatalf("write wireless config: %v", err)
	}
	if err := applier.Snapshot([]string{"wireless"}); err != nil {
		t.Fatalf("Snapshot error = %v", err)
	}
	if err := os.WriteFile(cfg, []byte("broken\n"), 0600); err != nil {
		t.Fatalf("commit broken config: %v", err)
	}
	sid, err := applier.StartApply([]string{"wireless"})
	if err != nil {
		t.Fatalf("StartApply error = %v", err)
	}
	if err := applier.Rollback(sid); err != nil {
		t.Fatalf("Rollback error = %v", err)
	}

	reloadCfg := fu.indexOf("uci", "reload_config")
	reloadNet := fu.indexOf("network", "reload")
	if reloadCfg < 0 {
		t.Error("Rollback must issue `uci reload_config` so rpcd re-reads /etc/config")
	}
	if reloadNet < 0 {
		t.Error("Rollback must issue `network reload` so netifd acts on the restored config")
	}
	restore := fu.indexOf("uci", "apply")
	if reloadCfg >= 0 && reloadCfg < restore {
		t.Error("the config must be restored before rpcd is asked to re-read it")
	}
	if reloadCfg >= 0 && reloadNet >= 0 && reloadCfg > reloadNet {
		t.Error("rpcd must re-read the config before netifd acts on it")
	}
	for _, c := range fu.calls {
		if c.method == "wifi" || c.method == "wifi up" || c.method == "wifi reload" {
			t.Errorf("Rollback must never run wifi: %s %s", c.path, c.method)
		}
	}
}

// A confirmed apply that fails at the end must leave nothing armed: the
// previous config is put back, the device is told to re-read it, and no snapshot
// is left behind to undo a LATER unrelated change.
func TestRealUCIApplyConfirm_ApplyAndConfirmRestoresWhenConfirmFails(t *testing.T) {
	t.Parallel()

	fu := &FakeUbusApply{LoginSID: "sess-aac", ConfirmErr: errors.New("confirm rejected")}
	applier, etcDir, runDir := newTestUCIApply(t, fu)
	cfg := filepath.Join(etcDir, "wireless")
	if err := os.WriteFile(cfg, []byte(snapshotWireless), 0600); err != nil {
		t.Fatalf("write wireless config: %v", err)
	}

	if err := applier.ApplyAndConfirm([]string{"wireless"}); err == nil {
		t.Fatal("expected ApplyAndConfirm to report the failed confirm")
	}
	if fu.indexOf("uci", "reload_config") < 0 || fu.indexOf("network", "reload") < 0 {
		t.Error("a failed ApplyAndConfirm must restore and re-read the previous config")
	}
	if entries, err := os.ReadDir(runDir); err == nil {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), uciSnapshotPrefix) {
				t.Errorf("snapshot %s survived a failed ApplyAndConfirm and can undo a later change", e.Name())
			}
		}
	}
	got, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(got) != snapshotWireless {
		t.Errorf("config after a failed ApplyAndConfirm = %q, want the pre-apply snapshot", got)
	}
}

func (f *FakeUbusApply) find(path, method string) *ubusCall {
	for i := range f.calls {
		if f.calls[i].path == path && f.calls[i].method == method {
			return &f.calls[i]
		}
	}
	return nil
}

func (f *FakeUbusApply) indexOf(path, method string) int {
	for i, c := range f.calls {
		if c.path == path && c.method == method {
			return i
		}
	}
	return -1
}
