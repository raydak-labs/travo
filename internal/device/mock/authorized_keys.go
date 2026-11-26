package mock

import (
	"context"
	"slices"
)

// ReadAuthorizedKeys returns root's authorized_keys from memory; nil reads as absent.
func (d *Device) ReadAuthorizedKeys(context.Context) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.AuthorizedKeysErr != nil {
		return nil, d.AuthorizedKeysErr
	}
	return slices.Clone(d.authorizedKeys), nil
}

// WriteAuthorizedKeys replaces the in-memory file.
func (d *Device) WriteAuthorizedKeys(_ context.Context, data []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.AuthorizedKeysErr != nil {
		return d.AuthorizedKeysErr
	}
	d.authorizedKeys = slices.Clone(data)
	return nil
}
