package cli

import (
	"context"
	"encoding/base64"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestWireGuardKeyStagingUsesStdinAndVerifiesReadback(t *testing.T) {
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdefghijklmnopqrstuv"))
	for _, tc := range []struct {
		name, out string
		code      int
		err       error
		want      bool
	}{
		{"verified", key + "\n", 0, nil, true},
		{"readback mismatch", "", 0, nil, false},
		{"sensitive runner error", key + "\n", 1, errors.New(key), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := Device{RunInput: func(_ context.Context, stdin, name string, args ...string) (string, int, error) {
				if name != "uci" || !slices.Equal(args, []string{"batch"}) {
					t.Fatal("expected uci batch without key in command arguments")
				}
				if stdin != "set network.wg0.private_key='"+key+"'\nget network.wg0.private_key\n" {
					t.Fatal("key and readback must use stdin")
				}
				return tc.out, tc.code, tc.err
			}}
			err := d.SetWireGuardKey(context.Background(), "wg0", "private_key", key)
			if (err == nil) != tc.want {
				t.Fatal("unexpected staging result")
			}
			if err != nil && (!errors.Is(err, errStageWireGuardKey) || strings.Contains(err.Error(), key)) {
				t.Fatal("expected redacted staging error")
			}
		})
	}
}
