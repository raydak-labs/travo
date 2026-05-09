package services

import "os/exec"

// CommandRunner abstracts OS command execution.
type CommandRunner interface {
	Run(name string, args ...string) ([]byte, error)
}

// RealCommandRunner executes real OS commands.
type RealCommandRunner struct{}

// Run executes a command and returns its output.
func (r *RealCommandRunner) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

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
