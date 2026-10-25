package platform

import (
	"errors"
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
type FakeUbusApply struct {
	calls      []ubusCall
	LoginSID   string
	LoginErr   error
	ApplyErr   error
	ConfirmErr error
}

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
	case "uci.apply":
		if f.ApplyErr != nil {
			return nil, f.ApplyErr
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

// indexOf returns the position of the first recorded call, or -1.
