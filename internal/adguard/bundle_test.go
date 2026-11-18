package adguard

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func withBundle(t *testing.T, r *rig, body []byte, pin []byte) {
	t.Helper()
	path := filepath.Join(r.dir, "bundle", "adguardhome-0.107.80-r1.apk")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(pin)
	r.svc.Bundle = Bundle{Path: path, SHA256: hex.EncodeToString(sum[:])}
}

func (p *recordingPkgs) InstallFile(ctx context.Context, path string) error {
	p.onInstall()
	return p.Mock.InstallFile(ctx, path)
}

func TestOfflineFirstInstallUsesTheVerifiedBundle(t *testing.T) {
	r := newRig(t)
	r.svc.Online = func(context.Context) bool { return false }
	var seenAtInstall []byte
	r.svc.Packages = &recordingPkgs{Mock: r.pkgs, onInstall: func() { seenAtInstall, _ = os.ReadFile(r.svc.Config) }}
	withBundle(t, r, []byte("apk bytes"), []byte("apk bytes"))
	if err := r.svc.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(r.pkgs.Files, []string{r.svc.Bundle.Path}) {
		t.Fatalf("installed files = %v", r.pkgs.Files)
	}
	if !bytes.Equal(seenAtInstall, Seed) {
		t.Fatal("the loopback seed must exist before the bundle installs")
	}
	if g := r.guards(t); len(g) != 0 {
		t.Fatalf("guards = %v", g)
	}
	if _, err := os.Stat(r.svc.Marker); err != nil {
		t.Fatal("the bundled install is the one automatic attempt")
	}
}

func TestMismatchedBundleIsNeverInstalled(t *testing.T) {
	r := newRig(t)
	r.svc.Online = func(context.Context) bool { return false }
	withBundle(t, r, []byte("tampered"), []byte("apk bytes"))
	if err := r.svc.Reconcile(context.Background()); !errors.Is(err, ErrOffline) {
		t.Fatalf("err = %v, want the online gate", err)
	}
	if len(r.pkgs.Files) != 0 {
		t.Fatalf("installed %v", r.pkgs.Files)
	}
	// Online, the feed package is used instead.
	r.svc.Online = func(context.Context) bool { return true }
	if err := r.svc.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(r.pkgs.Files) != 0 {
		t.Fatalf("installed %v from a mismatched bundle", r.pkgs.Files)
	}
	if _, err := r.pkgs.Version(context.Background(), Package); err != nil {
		t.Fatal("online install did not run")
	}
}

func TestNoPinMeansNoOfflinePath(t *testing.T) {
	if err := (Bundle{Path: "/tmp/x.apk"}).Verify(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v", err)
	}
}
