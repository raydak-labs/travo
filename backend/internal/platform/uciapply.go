package platform

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/auth"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
)

// UCIApplyConfirm stages UCI config changes using rpcd's apply+confirm flow
// (same as LuCI Save & Apply), so a rollback window exists and the device
// can revert if it crashes before confirm. User-driven wireless flows should
// start apply first and confirm only after the browser proves the router is
// still reachable on the new settings.
type UCIApplyConfirm interface {
	// StartApply commits staged UCI by: session login, copy configs to session
	// dir, uci apply (rollback timeout). Returns the rpcd session ID used for
	// later confirm. Configs are names like "wireless", "network", "system".
	StartApply(configs []string) (string, error)
	// Confirm finalizes a previously started apply session and discards its
	// snapshot: this is the success path.
	Confirm(sessionID string) error
	// Snapshot copies the named on-disk config files so a later Rollback can put
	// back what a committed mutation replaced. It must run BEFORE the mutation
	// commits, because after a commit uci revert is a no-op and rpcd's window has
	// already snapshotted the changed files.
	Snapshot(configs []string) error
	// Rollback restores the snapshot taken for sessionID and asks netifd to
	// re-read the restored config. It is the recovery path for a mutation that
	// was committed but must not stand. An empty sessionID means the snapshot
	// that no apply claimed yet. A missing snapshot is an ERROR, never a
	// reported success.
	Rollback(sessionID string) error
	// ApplyAndConfirm is retained for guarded internal flows that still need a
	// synchronous apply on the router itself.
	ApplyAndConfirm(configs []string) error
}

const (
	ApplyRollbackTimeout = 30
	defaultEtcConfigDir  = "/etc/config"
	defaultRpcdRunDir    = "/var/run/rpcd"

	// Snapshot layout, all under the rpcd run dir (a tmpfs on OpenWrt, so it
	// costs no flash and cannot survive a reboot, which is the intent: a
	// snapshot that outlived a power cut would undo an unrelated later edit).
	uciSnapshotPrefix  = "travo-snapshot-"
	uciSnapshotPending = "travo-snapshot-pending"
	// snapshotManifest lists the configs this snapshot covers: one line per
	// name, prefixed `present ` or `absent `. Without it Rollback would restore
	// only the files it happens to find and could not tell "was never there"
	// from "is missing right now".
	snapshotManifest = "manifest"
)

// snapshotDir returns the directory holding the snapshot for a session. The
// empty session is the PENDING snapshot: taken before a mutation, not yet
// associated with an apply.
func (r *RealUCIApplyConfirm) snapshotDir(sessionID string) string {
	if sessionID == "" {
		return filepath.Join(r.RPCDRunDir, uciSnapshotPending)
	}
	return filepath.Join(r.RPCDRunDir, uciSnapshotPrefix+sessionID)
}

// ErrApplyFailedMessage is what a caller sees when rpcd refuses an apply or a
// confirm. The ubus CLI's own text carries the call arguments, including the rpc
// session id (a bearer token), so it is logged redacted and never returned.
const ErrApplyFailedMessage = "The router could not apply the change. Please try again; " +
	"if it keeps failing, check the router logs."

// applyError is the caller-facing error: a fixed message that still unwraps to
// the cause, so errors.Is keeps working.
type applyError struct{ cause error }

func (e *applyError) Error() string { return ErrApplyFailedMessage }
func (e *applyError) Unwrap() error { return e.cause }

var sessionTokenRe = regexp.MustCompile(`("ubus_rpc_session"\s*:\s*")[^"]*(")`)

// redactSession blanks the rpc session id in a ubus failure text.
func redactSession(text string) string { return sessionTokenRe.ReplaceAllString(text, "${1}***${2}") }

// failApply logs the redacted detail and returns the fixed caller-facing error.
func (r *RealUCIApplyConfirm) failApply(stage string, cause error) error {
	r.logger().Error("uci "+stage+" failed", "err", redactSession(cause.Error()))
	return &applyError{cause: cause}
}

func (r *RealUCIApplyConfirm) logger() *slog.Logger {
	if r.Log != nil {
		return r.Log
	}
	return slog.Default()
}

// RealUCIApplyConfirm uses ubus session + rpcd uci apply/confirm.
type RealUCIApplyConfirm struct {
	ubus     ubus.Ubus
	password *auth.RootPassword
	// Log receives the redacted detail of a failed apply; nil means slog.Default.
	Log *slog.Logger
	// RPCDRunDir and etcConfigDir are struct fields (not constants) so the
	// apply/confirm flow can be exercised in tests against a temp directory.
	RPCDRunDir   string
	EtcConfigDir string
}

// NewRealUCIApplyConfirm returns a real applier that uses the given ubus and password holder.
func NewRealUCIApplyConfirm(ub ubus.Ubus, pw *auth.RootPassword) *RealUCIApplyConfirm {
	return &RealUCIApplyConfirm{
		ubus:         ub,
		password:     pw,
		RPCDRunDir:   defaultRpcdRunDir,
		EtcConfigDir: defaultEtcConfigDir,
	}
}

// Snapshot copies the named configs out of /etc/config before a mutation
// commits. This is OUR rollback, not rpcd's: rpcd snapshots at apply time, and
// the apply happens after the commit, so its window can only ever undo the
// change it was told to make (ADR 0002 §5).
//
// ONE pending snapshot is enough. Every wireless mutation runs under
// mutateWireless, which holds the wireless write lock for the whole
// snapshot → mutate → apply sequence, so no second snapshot can be taken in
// between; a queue would be dead code. The name says so out loud: a snapshot
// that is not pending is keyed to an apply session.
func (r *RealUCIApplyConfirm) Snapshot(configs []string) error {
	if len(configs) == 0 {
		return nil
	}
	dir := r.snapshotDir("")
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("uci snapshot: clear %s: %w", dir, err)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("uci snapshot: mkdir %s: %w", dir, err)
	}
	var manifest strings.Builder
	for _, name := range configs {
		// A config name becomes a file name under the snapshot dir and a path
		// back into /etc/config. Every caller passes a code constant, but a
		// name that could escape those two directories must not be honoured
		// just because no caller is supposed to send one.
		if name == "" || filepath.Base(name) != name {
			_ = os.RemoveAll(dir)
			return fmt.Errorf("uci snapshot: %q is not a plain config file name", name)
		}
		src := filepath.Join(r.EtcConfigDir, name)
		if _, err := os.Stat(src); err != nil {
			if !os.IsNotExist(err) {
				_ = os.RemoveAll(dir)
				return fmt.Errorf("uci snapshot: stat %s: %w", name, err)
			}
			// Absent before the mutation: Rollback deletes whatever the
			// mutation created, instead of leaving a file that did not exist.
			manifest.WriteString("absent " + name + "\n")
			continue
		}
		if err := copyFile(src, filepath.Join(dir, name)); err != nil {
			_ = os.RemoveAll(dir)
			return fmt.Errorf("uci snapshot: copy %s: %w", name, err)
		}
		manifest.WriteString("present " + name + "\n")
	}
	path := filepath.Join(dir, snapshotManifest)
	if err := os.WriteFile(path, []byte(manifest.String()), 0600); err != nil {
		_ = os.RemoveAll(dir)
		return fmt.Errorf("uci snapshot: write %s: %w", path, err)
	}
	return nil
}

// ErrNoSnapshot reports that a rollback was asked for a session that has no
// snapshot to restore. It is exported so callers can tell "there was nothing to
// put back" from "the restore failed" — the first is expected on some paths,
// the second must never be swallowed.
var ErrNoSnapshot = errors.New("no config snapshot for that session")

// Rollback restores the snapshot for sessionID over /etc/config and makes the
// device act on it. It is the recovery path for a mutation that was committed
// and must not stand — see ADR 0002 §5 for why rpcd's window cannot do this.
//
// Restoring files is not enough: rpcd and netifd keep their own view, so they
// are asked to re-read. `uci reload_config` refreshes rpcd's view of the config
// files; `network reload` makes netifd re-read /etc/config/network AND
// wireless. This is the ONE place in the codebase allowed to do that: ADR 0003
// forbids `wifi` / `wifi up` / `wifi reload` on the apply path (they are the
// ath11k/IPQ6018 driver-crash trigger), and recovery after a refused change is
// exactly the bounded-recovery exception it carves out. `wifi` is never used
// here — only `network reload`, which is not a driver reload.
//
// The rpcd window is cancelled (uci confirm) right after the restore. Leaving
// it armed would undo this restore at expiry: rpcd's snapshot is the ALREADY
// COMMITTED config, so its expiry would put the broken change back.
func (r *RealUCIApplyConfirm) Rollback(sessionID string) error {
	dir := r.snapshotDir(sessionID)
	raw, err := os.ReadFile(filepath.Join(dir, snapshotManifest))
	if err != nil {
		return fmt.Errorf("%w: session %q under %s: %w", ErrNoSnapshot, sessionID, dir, err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		state, name, ok := strings.Cut(strings.TrimSpace(line), " ")
		if !ok || name == "" {
			return fmt.Errorf("uci rollback: unreadable snapshot manifest line %q in %s", line, dir)
		}
		target := filepath.Join(r.EtcConfigDir, name)
		if state == "absent" {
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("uci rollback: remove %s: %w", target, err)
			}
			continue
		}
		if err := restoreFile(filepath.Join(dir, name), target); err != nil {
			return fmt.Errorf("uci rollback: restore %s: %w", name, err)
		}
	}
	// Cancel rpcd's window before reloading: an armed window would fire at
	// expiry and overwrite the restore with the config it snapshotted — the
	// already-committed one.
	var windowErr error
	if sessionID != "" {
		windowErr = r.Confirm(sessionID)
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("uci rollback: clearing the snapshot in %s: %w", dir, err)
	}
	// Order matters: rpcd re-reads the files, then netifd acts on them.
	if _, err := r.ubus.Call("uci", "reload_config", nil); err != nil {
		return fmt.Errorf("uci rollback: uci reload_config: %w", err)
	}
	if _, err := r.ubus.Call("network", "reload", nil); err != nil {
		return fmt.Errorf("uci rollback: network reload: %w", err)
	}
	if windowErr != nil {
		// The files ARE back and the device has acted on them, so this is not a
		// failed rollback — but rpcd may still overwrite them at its own
		// deadline, and the operator has to hear that it might.
		return fmt.Errorf("uci rollback: the previous config was restored, but rpcd's "+
			"rollback window for session %s could not be cancelled and may overwrite it: %w",
			sessionID, windowErr)
	}
	return nil
}

// restoreFile is copyFile for the rollback direction: a snapshot entry that is
// missing is an ERROR. copyFile skips missing sources (right when staging an
// apply, where a missing config is normal), but here a missing entry means
// there is nothing to put back and reporting success would strand the operator.
func restoreFile(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("snapshot entry %s is missing: %w", src, err)
	}
	return copyFile(src, dst)
}

// applyMarkerSection names the throwaway section StartApply adds and deletes.
const applyMarkerSection = "travo_apply"

// StartApply opens an rpcd rollback apply and returns the session ID.
//
// rpcd's `uci apply` acts only on the configs that have a pending change in the
// calling session. Older rpcd also picked up files copied into
// /var/run/rpcd/uci-<session id>, which is what this used to do; rpcd
// 2026.10.04 names that directory after a private random value instead, finds
// nothing, and the ubus CLI reports "(No response)" (#220). The only supported
// way to put a config into the session's pending set is to change it through
// the session.
//
// Travo has already committed its change by the time it gets here (ADR 0002
// §5.0), so the real edit is not in the session. Each config therefore gets a
// marker section added and deleted again through the session: the pending delta
// is non-empty, which makes rpcd apply and snapshot the config, and its net
// effect on the config is nothing.
func (r *RealUCIApplyConfirm) StartApply(configs []string) (string, error) {
	if len(configs) == 0 {
		return "", nil
	}
	sid, err := r.sessionLogin()
	if err != nil || sid == "" {
		return "", fmt.Errorf("uci apply: no session (login failed): %w", err)
	}
	// Anything pending in the session when apply does not start is dropped, or a
	// later reuse of the session would apply a marker nobody asked for.
	staged := make([]string, 0, len(configs))
	applyStarted := false
	defer func() {
		if !applyStarted {
			r.revertSession(sid, staged)
		}
	}()
	for _, name := range configs {
		// A config that does not exist (mwan3 not installed) has nothing to
		// apply, as before.
		if _, err := os.Stat(filepath.Join(r.EtcConfigDir, name)); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("uci apply: stat %s: %w", name, err)
		}
		// Recorded before staging: a marker added but not deleted has to be
		// reverted too.
		staged = append(staged, name)
		if err := r.stageMarker(sid, name); err != nil {
			return "", err
		}
	}
	if len(staged) == 0 {
		return "", nil
	}
	applyArgs := map[string]any{
		"ubus_rpc_session": sid,
		"rollback":         true,
		"timeout":          ApplyRollbackTimeout,
	}
	if _, err := r.ubus.Call("uci", "apply", applyArgs); err != nil {
		return "", r.failApply("apply", err)
	}
	applyStarted = true
	// The apply still buys rpcd's crash-time rollback, but Snapshot is what puts
	// the previous config back. Claiming the pending snapshot names it after this
	// session, so ConfirmApply can restore it.
	r.claimPendingSnapshot(sid)
	return sid, nil
}

// stageMarker leaves config with a pending, net-empty change in the session.
func (r *RealUCIApplyConfirm) stageMarker(sid, config string) error {
	add := map[string]any{
		"config":           config,
		"type":             applyMarkerSection,
		"name":             applyMarkerSection,
		"ubus_rpc_session": sid,
	}
	if _, err := r.ubus.Call("uci", "add", add); err != nil {
		return fmt.Errorf("uci apply: add marker to %s in the session: %w", config, err)
	}
	del := map[string]any{
		"config":           config,
		"section":          applyMarkerSection,
		"ubus_rpc_session": sid,
	}
	if _, err := r.ubus.Call("uci", "delete", del); err != nil {
		return fmt.Errorf("uci apply: delete marker from %s in the session: %w", config, err)
	}
	return nil
}

// revertSession drops the pending changes StartApply made, best effort: the
// caller is already returning the error that matters.
func (r *RealUCIApplyConfirm) revertSession(sid string, configs []string) {
	for _, name := range configs {
		args := map[string]any{"config": name, "ubus_rpc_session": sid}
		if _, err := r.ubus.Call("uci", "revert", args); err != nil {
			slog.Warn("uci apply: could not revert the staged marker", "config", name, "err", err)
		}
	}
}

// claimPendingSnapshot moves the pending snapshot onto the apply session it
// belongs to. A mutation that never called Snapshot simply has none.
func (r *RealUCIApplyConfirm) claimPendingSnapshot(sessionID string) {
	pending := r.snapshotDir("")
	if _, err := os.Stat(filepath.Join(pending, snapshotManifest)); err != nil {
		return
	}
	if err := os.Rename(pending, r.snapshotDir(sessionID)); err != nil {
		slog.Error("uci apply: cannot key the config snapshot to the session", "session", sessionID, "err", err)
	}
}

// Confirm finalizes a previously started apply session.
func (r *RealUCIApplyConfirm) Confirm(sessionID string) error {
	if sessionID == "" {
		return fmt.Errorf("uci confirm: empty session id")
	}
	confirmArgs := map[string]any{
		"ubus_rpc_session": sessionID,
	}
	if _, err := r.ubus.Call("uci", "confirm", confirmArgs); err != nil {
		return r.failApply("confirm", err)
	}
	// Success: this session's snapshot is dead weight, and a stale one must not
	// be able to undo a LATER unrelated change.
	if err := os.RemoveAll(r.snapshotDir(sessionID)); err != nil {
		return fmt.Errorf("uci confirm: clearing the snapshot of session %s: %w", sessionID, err)
	}
	return nil
}

// ApplyAndConfirm implements UCIApplyConfirm. A guarded internal flow has no
// browser to confirm, so it snapshots the configs first, then applies and
// confirms. If either half fails, the change it already committed is rolled back
// by hand: there is no session left to time out, and `uci revert` cannot undo a
// commit.
func (r *RealUCIApplyConfirm) ApplyAndConfirm(configs []string) error {
	// Consistent with StartApply: nothing to apply is a successful no-op, not an
	// error (an empty list must not fail later in Confirm with an empty session).
	if len(configs) == 0 {
		return nil
	}
	if err := r.Snapshot(configs); err != nil {
		return err
	}
	sid, err := r.StartApply(configs)
	if err != nil {
		// StartApply failed AFTER the mutation committed and there is no session
		// to roll back, so the pending snapshot is the only way back.
		return r.rollbackOrJoin("apply", "", err)
	}
	if sid == "" {
		// None of the configs exist, so nothing was applied and there is nothing
		// to confirm. Discard the snapshot the way a confirmed apply would.
		return os.RemoveAll(r.snapshotDir(""))
	}
	if err := r.Confirm(sid); err != nil {
		return r.rollbackOrJoin("confirm", sid, err)
	}
	return nil
}

// rollbackOrJoin reports cause with the rollback outcome attached. The cause
// stays the unwrappable error — it is what the caller must act on — but a
// rollback that failed is stated loudly, because that is the case where the
// previous config is NOT back on disk.
func (r *RealUCIApplyConfirm) rollbackOrJoin(stage, sessionID string, cause error) error {
	if rbErr := r.Rollback(sessionID); rbErr != nil {
		return fmt.Errorf("%w (rolling back to the previous config after a failed %s also failed: %w)",
			cause, stage, rbErr)
	}
	return cause
}

func (r *RealUCIApplyConfirm) sessionLogin() (string, error) {
	args := map[string]any{
		"username": "root",
		"password": r.password.Get(),
	}
	resp, err := r.ubus.Call("session", "login", args)
	if err != nil {
		return "", err
	}
	return ubus.ExtractSessionID(resp), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // skip missing config (same as setup script)
		}
		return err
	}
	defer func() { _ = in.Close() }()
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Sync()
}
