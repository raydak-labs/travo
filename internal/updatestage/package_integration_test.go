//go:build integration

package updatestage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
)

// The booted package suite supplies the real mkpkg output and a throwaway
// legacy-format minisign signature, with usign on the OpenWrt image.
func TestRealReleasePackage(t *testing.T) {
	input := os.Getenv("TRAVO_STAGE_PACKAGE_FIXTURE")
	if input == "" {
		t.Skip("requires booted package suite")
	}
	ctx := context.Background()
	manifest := filepath.Join(input, "checksums.txt")
	signature := filepath.Join(input, "checksums.txt.sig")
	key := filepath.Join(input, "minisign.pub")
	verifier := cli.New()
	if err := verifier.VerifyRelease(ctx, manifest, signature, key); err != nil {
		t.Fatal(err)
	}
	if err := verifier.VerifyRelease(ctx, manifest, filepath.Join(input, "bad.sig"), key); err == nil {
		t.Fatal("accepted signature from a different key")
	}
	signed, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := extract(ctx, filepath.Join(input, "release.tar.gz"), t.TempDir(), signed, os.Getenv("TRAVO_STAGE_PACKAGE_VERSION")); err != nil {
		t.Fatal(err)
	}
}
