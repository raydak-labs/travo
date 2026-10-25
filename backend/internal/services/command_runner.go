package services

import "github.com/openwrt-travel-gui/backend/internal/execx"

// CommandRunner abstracts OS command execution.
type CommandRunner interface {
	Run(name string, args ...string) ([]byte, error)
}

// RealCommandRunner executes real OS commands with a bounded runtime so a
// hung command can never pin a handler goroutine forever.
type RealCommandRunner struct{}

// Run executes a command and returns its output.
func (r *RealCommandRunner) Run(name string, args ...string) ([]byte, error) {
	return execx.Output(execx.Slow, name, args...)
}
