package mock

import "context"

// ReadCrontab returns root's crontab from memory.
func (d *Device) ReadCrontab(context.Context) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.crontab, d.CrontabErr
}

// WriteCrontab replaces the in-memory crontab.
func (d *Device) WriteCrontab(_ context.Context, content string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.CrontabErr != nil {
		return d.CrontabErr
	}
	d.crontab = content
	return nil
}
