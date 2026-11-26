package cli

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func TestRenewLeaseCallsNetifdAndAcceptsEmptyReply(t *testing.T) {
	for _, tc := range []struct {
		name, out string
		err       error
		ok        bool
	}{
		{"empty reply", "", nil, true},
		{"unexpected reply", "oops", nil, false},
		{"runner error", "", errors.New("Command failed: Not found"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			d := Device{Run: func(_ context.Context, name string, args ...string) (string, int, error) {
				got = append([]string{name}, args...)
				return tc.out, 0, tc.err
			}}
			err := d.RenewLease(t.Context(), "wwan")
			if (err == nil) != tc.ok {
				t.Fatal(err)
			}
			if !slices.Equal(got, []string{"ubus", "call", "network.interface.wwan", "renew"}) {
				t.Fatal(got)
			}
		})
	}
	d := Device{Run: func(context.Context, string, ...string) (string, int, error) {
		t.Fatal("ran with an invalid name")
		return "", 0, nil
	}}
	if err := d.RenewLease(t.Context(), "wwan; reboot"); err == nil {
		t.Fatal("accepted an invalid interface name")
	}
}
