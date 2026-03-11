package services

// MockCommandRunner implements CommandRunner for testing.
type MockCommandRunner struct {
	Output []byte
	Err    error
}

// Run returns the configured output and error.
func (m *MockCommandRunner) Run(_ string, _ ...string) ([]byte, error) {
	return m.Output, m.Err
}
