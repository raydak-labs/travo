package platform

import (
	"errors"
	"sort"
)

// MockCommandRunner implements CommandRunner for testing.
type MockCommandRunner struct {
	Output  []byte
	Err     error
	RunFunc func(name string, args ...string) ([]byte, error)
}

// Run returns the configured output and error.
func (m *MockCommandRunner) Run(name string, args ...string) ([]byte, error) {
	if m.RunFunc != nil {
		return m.RunFunc(name, args...)
	}
	return m.Output, m.Err
}

// Snapshot is a no-op for the no-op applier.
func (NoopUCIApplyConfirm) Snapshot(_ []string) error { return nil }

// Rollback is a no-op for the no-op applier.
func (NoopUCIApplyConfirm) Rollback(_ string) error { return nil }

// NoopUCIApplyConfirm does nothing (for tests).
type NoopUCIApplyConfirm struct{}

// StartApply is a no-op.
func (NoopUCIApplyConfirm) StartApply(_ []string) (string, error) {
	return "", nil
}

// Confirm is a no-op.
func (NoopUCIApplyConfirm) Confirm(_ string) error {
	return nil
}

// ApplyAndConfirm is a no-op.
func (NoopUCIApplyConfirm) ApplyAndConfirm(_ []string) error {
	return nil
}

// FuncCommandRunner lets tests provide a function-based command runner.
type FuncCommandRunner struct {
	RunFunc func(name string, args ...string) ([]byte, error)
}

func (f *FuncCommandRunner) Run(name string, args ...string) ([]byte, error) {
	return f.RunFunc(name, args...)
}

// FakeUbusApply records uci/session ubus calls for RealUCIApplyConfirm tests.
//
// It models the one rpcd behaviour that broke production: `uci apply` acts only
// on configs that have a pending change in THAT session's delta, and answers
// "No response" (UBUS_STATUS_NO_DATA) when there is none. Files copied into
// /var/run/rpcd by hand do not count: rpcd names its session directory after a
// private random value, not the session id (rpcd 2026.10.04, device finding in
// ADR 0013), so only changes made through the session's own uci calls reach it.
type FakeUbusApply struct {
	calls      []ubusCall
	LoginSID   string
	LoginErr   error
	ApplyErr   error
	ConfirmErr error
	AddErr     error
	// DeleteErrFor fails uci.delete for the named configs.
	DeleteErrFor map[string]error

	// pending counts the net pending changes per session and config.
	pending map[string]map[string]int
}

// errNoResponse is what the ubus CLI prints when rpcd answers NO_DATA.
var errNoResponse = errors.New("ubus call uci apply: Command failed (No response)")

type ubusCall struct {
	path   string
	method string
	args   map[string]any
}

func (f *FakeUbusApply) Call(path, method string, args map[string]any) (map[string]any, error) {
	f.calls = append(f.calls, ubusCall{path: path, method: method, args: args})
	switch path + "." + method {
	case "session.login":
		if f.LoginErr != nil {
			return nil, f.LoginErr
		}
		sid := f.LoginSID
		if sid == "" {
			sid = "sid-1"
		}
		return map[string]any{"ubus_rpc_session": sid, "timeout": float64(300)}, nil
	case "uci.add":
		if f.AddErr != nil {
			return nil, f.AddErr
		}
		f.touch(args, 1)
		return map[string]any{"section": args["name"]}, nil
	case "uci.delete":
		if cfg, _ := args["config"].(string); f.DeleteErrFor[cfg] != nil {
			return nil, f.DeleteErrFor[cfg]
		}
		f.touch(args, 1)
		return map[string]any{}, nil
	case "uci.revert":
		sid, _ := args["ubus_rpc_session"].(string)
		config, _ := args["config"].(string)
		delete(f.pending[sid], config)
		return map[string]any{}, nil
	case "uci.apply":
		if f.ApplyErr != nil {
			return nil, f.ApplyErr
		}
		if !f.hasPending(args) {
			return nil, errNoResponse
		}
		return map[string]any{}, nil
	case "uci.confirm":
		if f.ConfirmErr != nil {
			return nil, f.ConfirmErr
		}
		return map[string]any{}, nil
	case "uci.reload_config", "network.reload":
		return map[string]any{}, nil
	}
	return nil, errors.New("ubus: unexpected call " + path + "." + method)
}

// touch records one pending change for the session and config named in args.
func (f *FakeUbusApply) touch(args map[string]any, n int) {
	sid, _ := args["ubus_rpc_session"].(string)
	config, _ := args["config"].(string)
	if f.pending == nil {
		f.pending = map[string]map[string]int{}
	}
	if f.pending[sid] == nil {
		f.pending[sid] = map[string]int{}
	}
	f.pending[sid][config] += n
}

// hasPending reports whether the session in args has any pending change.
func (f *FakeUbusApply) hasPending(args map[string]any) bool {
	sid, _ := args["ubus_rpc_session"].(string)
	return len(f.pending[sid]) > 0
}

// PendingConfigs returns the configs the session has pending changes for.
func (f *FakeUbusApply) PendingConfigs(sid string) []string {
	var out []string
	for c := range f.pending[sid] {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// indexOf returns the position of the first recorded call, or -1.
