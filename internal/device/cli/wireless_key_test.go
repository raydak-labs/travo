package cli

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestWirelessKeyFailureKeepsSecretOutOfDiagnostics(t *testing.T) {
	const key = "private-test-key"
	d := Device{RunInput: func(_ context.Context, stdin, name string, args ...string) (string, int, error) {
		if name != "uci" || !slices.Equal(args, []string{"batch"}) || !strings.Contains(stdin, key) {
			t.Fatal("expected uci batch with the key on stdin")
		}
		return key, 1, errors.New(key)
	}}
	if err := d.SetWirelessKey(context.Background(), "wwan", key); err != errStageWirelessKey {
		t.Fatal("expected a generic staging error")
	}
}
