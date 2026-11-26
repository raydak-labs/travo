//go:build integration

package cli

import (
	"context"
	"testing"
)

// Needs a BOOTED container (procd, ubusd, rpcd): see mise task test:integration.
// With an empty shadow field rpcd accepts ANY password, so RootPasswordSet must be
// read first; that is why first-run never tries a login.
func TestRealRootAccount(t *testing.T) {
	ctx := context.Background()
	d := New()
	const pw = "s3cret-pass"

	if set, err := d.RootPasswordSet(ctx); err != nil || set {
		t.Fatalf("fresh image: set=%v err=%v (the image ships an empty root password)", set, err)
	}
	if err := d.SetRootPassword(ctx, pw); err != nil {
		t.Fatal(err)
	}
	if set, err := d.RootPasswordSet(ctx); err != nil || !set {
		t.Fatalf("after passwd: set=%v err=%v", set, err)
	}
	if ok, err := d.VerifyRootPassword(ctx, pw); err != nil || !ok {
		t.Fatalf("correct password: ok=%v err=%v", ok, err)
	}
	for _, wrong := range []string{"wrong-password", pw + "x", "", `"quote'; $(id)`} {
		if ok, err := d.VerifyRootPassword(ctx, wrong); err != nil || ok {
			t.Fatalf("wrong %q: ok=%v err=%v", wrong, ok, err)
		}
	}
}
