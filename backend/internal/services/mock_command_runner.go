package services

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
