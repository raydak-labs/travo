package services

import (
	"errors"
	"os"
	"path/filepath"
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

// fakeUbusApply records uci/session ubus calls for RealUCIApplyConfirm tests.
type fakeUbusApply struct {
	calls      []ubusCall
	loginSID   string
	loginErr   error
	applyErr   error
	confirmErr error
}

type ubusCall struct {
	path   string
	method string
	args   map[string]any
}

func (f *fakeUbusApply) Call(path, method string, args map[string]any) (map[string]any, error) {
	f.calls = append(f.calls, ubusCall{path: path, method: method, args: args})
	switch path + "." + method {
	case "session.login":
		if f.loginErr != nil {
			return nil, f.loginErr
		}
		sid := f.loginSID
		if sid == "" {
			sid = "sid-1"
		}
		return map[string]any{"ubus_rpc_session": sid, "timeout": float64(300)}, nil
	case "uci.apply":
		if f.applyErr != nil {
			return nil, f.applyErr
		}
		return map[string]any{}, nil
	case "uci.confirm":
		if f.confirmErr != nil {
			return nil, f.confirmErr
		}
		return map[string]any{}, nil
	case "uci.reload_config", "network.reload":
		return map[string]any{}, nil
	}
	return nil, errors.New("ubus: unexpected call " + path + "." + method)
}

func (f *fakeUbusApply) find(path, method string) *ubusCall {
	for i := range f.calls {
		if f.calls[i].path == path && f.calls[i].method == method {
			return &f.calls[i]
		}
	}
	return nil
}

// newTestUCIApply builds a RealUCIApplyConfirm wired to temp dirs and a fake ubus.
func newTestUCIApply(t *testing.T, fu *fakeUbusApply) (*RealUCIApplyConfirm, string, string) {
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
	applier.etcConfigDir = etcDir
	applier.rpcdRunDir = runDir
	return applier, etcDir, runDir
}

func TestRealUCIApplyConfirm_StartApplyEmptyListIsNoop(t *testing.T) {
	t.Parallel()

	fu := &fakeUbusApply{}
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

	fu := &fakeUbusApply{}
	applier, _, _ := newTestUCIApply(t, fu)

	if err := applier.ApplyAndConfirm([]string{}); err != nil {
		t.Fatalf("ApplyAndConfirm([]) error = %v, want nil", err)
	}
	if len(fu.calls) != 0 {
		t.Errorf("expected no ubus calls for empty config list, got %d", len(fu.calls))
	}
}

func TestRealUCIApplyConfirm_StartApplyStagesConfigsWithRollbackWindow(t *testing.T) {
	t.Parallel()

	fu := &fakeUbusApply{loginSID: "sess-42"}
	applier, etcDir, runDir := newTestUCIApply(t, fu)

	if err := os.WriteFile(filepath.Join(etcDir, "mwan3"), []byte("config mwan3 'x'\n"), 0600); err != nil {
		t.Fatalf("write mwan3 config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(etcDir, "network"), []byte("config network 'x'\n"), 0600); err != nil {
		t.Fatalf("write network config: %v", err)
	}

	sid, err := applier.StartApply([]string{"network", "mwan3"})
	if err != nil {
		t.Fatalf("StartApply error = %v", err)
	}
	if sid != "sess-42" {
		t.Fatalf("session = %q, want sess-42", sid)
	}

	sessionDir := filepath.Join(runDir, "uci-"+sid)
	for _, name := range []string{"network", "mwan3"} {
		if _, statErr := os.Stat(filepath.Join(sessionDir, name)); statErr != nil {
			t.Errorf("expected staged %s in session dir: %v", name, statErr)
		}
	}

	applyCall := fu.find("uci", "apply")
	if applyCall == nil {
		t.Fatal("expected uci.apply call")
	}
	if applyCall.args["rollback"] != true {
		t.Errorf("rollback = %v, want true", applyCall.args["rollback"])
	}
	if applyCall.args["timeout"] != uciApplyRollbackTimeout {
		t.Errorf("timeout = %v, want %d", applyCall.args["timeout"], uciApplyRollbackTimeout)
	}
	if applyCall.args["ubus_rpc_session"] != "sess-42" {
		t.Errorf("ubus_rpc_session = %v, want sess-42", applyCall.args["ubus_rpc_session"])
	}
	if fu.find("uci", "confirm") != nil {
		t.Error("StartApply must not confirm before the caller verifies")
	}
}

func TestRealUCIApplyConfirm_StartApplySkipsMissingConfig(t *testing.T) {
	t.Parallel()

	fu := &fakeUbusApply{}
	applier, _, runDir := newTestUCIApply(t, fu)

	sid, err := applier.StartApply([]string{"mwan3"})
	if err != nil {
		t.Fatalf("StartApply error = %v (missing configs are skipped)", err)
	}
	if _, statErr := os.Stat(filepath.Join(runDir, "uci-"+sid)); statErr != nil {
		t.Errorf("session dir should be kept for a started apply: %v", statErr)
	}
}

func TestRealUCIApplyConfirm_StartApplyCleansSessionDirOnApplyFailure(t *testing.T) {
	t.Parallel()

	fu := &fakeUbusApply{loginSID: "sess-fail", applyErr: errors.New("apply boom")}
	applier, etcDir, runDir := newTestUCIApply(t, fu)

	if err := os.WriteFile(filepath.Join(etcDir, "mwan3"), []byte("config mwan3 'x'\n"), 0600); err != nil {
		t.Fatalf("write mwan3 config: %v", err)
	}

	if _, err := applier.StartApply([]string{"mwan3"}); err == nil {
		t.Fatal("expected StartApply to fail when uci.apply fails")
	}
	if _, statErr := os.Stat(filepath.Join(runDir, "uci-"+"sess-fail")); !os.IsNotExist(statErr) {
		t.Errorf("expected session dir cleanup after failed apply, stat err = %v", statErr)
	}
}

func TestRealUCIApplyConfirm_StartApplyCleansSessionDirOnCopyFailure(t *testing.T) {
	t.Parallel()

	fu := &fakeUbusApply{loginSID: "sess-copy"}
	applier, etcDir, runDir := newTestUCIApply(t, fu)

	// A directory named like a config makes the staged copy fail.
	if err := os.MkdirAll(filepath.Join(etcDir, "mwan3"), 0700); err != nil {
		t.Fatalf("create bogus mwan3 dir: %v", err)
	}

	if _, err := applier.StartApply([]string{"mwan3"}); err == nil {
		t.Fatal("expected StartApply to fail when staging a config fails")
	}
	if fu.find("uci", "apply") != nil {
		t.Error("uci.apply must not run when staging fails")
	}
	if _, statErr := os.Stat(filepath.Join(runDir, "uci-sess-copy")); !os.IsNotExist(statErr) {
		t.Errorf("expected session dir cleanup after staging failure, stat err = %v", statErr)
	}
}

func TestRealUCIApplyConfirm_StartApplyFailsWithoutSession(t *testing.T) {
	t.Parallel()

	fu := &fakeUbusApply{loginErr: errors.New("login denied")}
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

	fu := &fakeUbusApply{}
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

	fu := &fakeUbusApply{loginSID: "sess-9"}
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

func TestRealUCIApplyConfirm_DefaultsToDevicePaths(t *testing.T) {
	t.Parallel()

	applier := NewRealUCIApplyConfirm(&fakeUbusApply{}, auth.NewRootPassword())
	if applier.rpcdRunDir != "/var/run/rpcd" {
		t.Errorf("rpcdRunDir = %q, want /var/run/rpcd", applier.rpcdRunDir)
	}
	if applier.etcConfigDir != "/etc/config" {
		t.Errorf("etcConfigDir = %q, want /etc/config", applier.etcConfigDir)
	}
}

// indexOf returns the position of the first recorded call, or -1.
func (f *fakeUbusApply) indexOf(path, method string) int {
	for i, c := range f.calls {
		if c.path == path && c.method == method {
			return i
		}
	}
	return -1
}

const snapshotWireless = "config wifi-device 'radio0'\n\tenabled '1'\n"

// The hardware finding: Travo commits the UCI change BEFORE either rollback
// mechanism runs, so rpcd's window snapshots the already-changed config and
// `uci revert` is a no-op on a committed delta. A refused confirm therefore left
// the broken profile enabled. Only a file-level snapshot taken BEFORE the
// mutation commits can put the previous config back.
func TestRealUCIApplyConfirm_RollbackRestoresACommittedConfig(t *testing.T) {
	t.Parallel()

	fu := &fakeUbusApply{loginSID: "sess-rb"}
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

	fu := &fakeUbusApply{loginSID: "sess-ok"}
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

	fu := &fakeUbusApply{}
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

	fu := &fakeUbusApply{loginSID: "sess-load"}
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

	fu := &fakeUbusApply{loginSID: "sess-aac", confirmErr: errors.New("confirm rejected")}
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
