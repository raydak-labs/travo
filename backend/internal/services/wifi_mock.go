package services

// NoopWifiReloader does nothing (for tests).
type NoopWifiReloader struct{}

// Reload is a no-op.
func (r *NoopWifiReloader) Reload() error { return nil }
