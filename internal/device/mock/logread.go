package mock

import (
	"context"
	_ "embed"
)

// systemLog is the AXT1800 `logread` capture (testdata/PROVENANCE.md).
//
//go:embed testdata/logread.txt
var systemLog string

// ReadSystemLog answers `logread` with the capture, or LogErr.
func (d *Device) ReadSystemLog(context.Context) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.LogErr != nil {
		return "", d.LogErr
	}
	return systemLog, nil
}
