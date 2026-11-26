package cli

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestWANPasswordGoesThroughStdinOnly(t *testing.T) {
	const pw = "it's s3cret"
	d := Device{RunInput: func(_ context.Context, stdin, name string, args ...string) (string, int, error) {
		if name != "uci" || !slices.Equal(args, []string{"batch"}) {
			t.Fatalf("argv = %s %v", name, args)
		}
		if !strings.Contains(stdin, `set network.wan.password='it'\''s s3cret'`) {
			t.Fatal("stdin did not quote the password")
		}
		return pw + "\n", 0, nil
	}}
	if err := d.SetWANPassword(context.Background(), pw); err != nil {
		t.Fatal(err)
	}
}

func TestDDNSPasswordStagesTravoSectionOnStdin(t *testing.T) {
	const pw = "token-x"
	d := Device{RunInput: func(_ context.Context, stdin, name string, args ...string) (string, int, error) {
		if name != "uci" || !slices.Equal(args, []string{"batch"}) || !strings.HasPrefix(stdin, "set ddns.travo.password='token-x'") {
			t.Fatalf("argv = %s %v", name, args)
		}
		return pw + "\n", 0, nil
	}}
	if err := d.SetDDNSPassword(context.Background(), pw); err != nil {
		t.Fatal(err)
	}
	if err := d.SetDDNSPassword(context.Background(), ""); err != errStageDDNSPassword {
		t.Fatal("empty token accepted")
	}
}

func TestWANPasswordFailureKeepsSecretOutOfErrors(t *testing.T) {
	const pw = "private-test-pw"
	d := Device{RunInput: func(context.Context, string, string, ...string) (string, int, error) {
		return pw, 1, errors.New(pw)
	}}
	if err := d.SetWANPassword(context.Background(), pw); err != errStageWANPassword {
		t.Fatal("expected a generic staging error")
	}
	for _, bad := range []string{"", "line\nbreak", strings.Repeat("x", 129)} {
		if err := d.SetWANPassword(context.Background(), bad); err != errStageWANPassword {
			t.Fatalf("%q accepted", bad)
		}
	}
}
