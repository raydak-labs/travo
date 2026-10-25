package system

import (
	"archive/tar"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/guard"
)

// RestoreBackup rewrites /etc/config, so a crash guard must exist and must
// survive a failed restore — the marker is the only record that the config was
// left in an unknown state.
func TestRestoreBackup_WritesAndKeepsGuardOnFailure(t *testing.T) {
	svc, dir := newGuardedSystemService(t)
	guard := guard.In(dir, restoreGuard).Path()

	// sysupgrade does not exist on the test host, so the restore fails; the
	// guard must still be on disk afterwards.
	path := filepath.Join(dir, "backup.tar.gz")
	if err := os.WriteFile(path, genuineBackupArchive(t), 0o600); err != nil {
		t.Fatalf("write backup: %v", err)
	}
	if err := svc.RestoreBackup(path); err == nil {
		t.Fatal("RestoreBackup reported success although sysupgrade is absent")
	}

	if _, err := os.Stat(guard); err != nil {
		t.Fatalf("expected a crash guard at %s after a failed restore: %v", guard, err)
	}
}

// A genuine backup passes validation: the allowlist has to accept what
// CreateBackup's counterpart (sysupgrade -b) actually produces, or restore is
// simply broken.
func TestValidateRestoreArchive_AcceptsGenuineBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.tar.gz")
	if err := os.WriteFile(path, genuineBackupArchive(t), 0o600); err != nil {
		t.Fatalf("write backup: %v", err)
	}
	if err := ValidateRestoreArchive(path); err != nil {
		t.Errorf("a genuine sysupgrade backup was rejected: %v", err)
	}
}

// sysupgrade -r extracts the tarball at /, so every one of these members turns
// "restore my configuration" into "run this as root".
func TestValidateRestoreArchive_RejectsUnsafeMembers(t *testing.T) {
	cases := []struct {
		name    string
		headers []*tar.Header
		bodies  map[string]string
		wantErr string
	}{
		{
			name: "absolute path",
			headers: []*tar.Header{
				{Name: "/etc/config/network", Typeflag: tar.TypeReg, Size: 3},
			},
			bodies:  map[string]string{"/etc/config/network": "abc"},
			wantErr: "not a relative path",
		},
		{
			name: "parent traversal",
			headers: []*tar.Header{
				{Name: "etc/config/../../../etc/crontabs/root", Typeflag: tar.TypeReg, Size: 3},
			},
			bodies:  map[string]string{"etc/config/../../../etc/crontabs/root": "abc"},
			wantErr: "escapes the archive root",
		},
		{
			name: "symlink to a startup script",
			headers: []*tar.Header{
				{Name: "etc/config/rc.local", Typeflag: tar.TypeSymlink, Linkname: "/etc/init.d/travo", Size: 0},
			},
			wantErr: "only regular files are allowed",
		},
		{
			name: "hardlink",
			headers: []*tar.Header{
				{Name: "etc/config/hard", Typeflag: tar.TypeLink, Linkname: "etc/config/network", Size: 0},
			},
			wantErr: "only regular files are allowed",
		},
		{
			name: "device node",
			headers: []*tar.Header{
				{Name: "etc/config/dev", Typeflag: tar.TypeChar, Size: 0},
			},
			wantErr: "only regular files are allowed",
		},
		{
			name: "not a configuration backup",
			headers: []*tar.Header{
				{Name: "etc/some-other/file", Typeflag: tar.TypeReg, Size: 3},
			},
			wantErr: "not a configuration backup",
		},
		{
			name: "member larger than the cap",
			headers: []*tar.Header{
				{Name: "etc/config/network", Typeflag: tar.TypeReg, Size: maxRestoreMemberBytes + 1},
			},
			wantErr: "larger than",
		},
		{
			name: "total uncompressed size over the cap",
			headers: func() []*tar.Header {
				var hs []*tar.Header
				for i := range 16 {
					hs = append(hs, &tar.Header{
						Name:     fmt.Sprintf("etc/config/big%d", i),
						Typeflag: tar.TypeReg,
						Size:     maxRestoreMemberBytes,
					})
				}
				return hs
			}(),
			wantErr: "uncompressed size exceeds",
		},
		{
			name: "too many members",
			headers: func() []*tar.Header {
				var hs []*tar.Header
				for i := 0; i <= maxRestoreArchiveMembers; i++ {
					hs = append(hs, &tar.Header{
						Name:     fmt.Sprintf("etc/config/f%d", i),
						Typeflag: tar.TypeReg,
						Size:     1,
					})
				}
				return hs
			}(),
			wantErr: "more than",
		},
		{
			name:    "empty archive",
			headers: nil,
			wantErr: "archive is empty",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "evil.tar.gz")
			if err := os.WriteFile(path, tarGzArchive(t, tc.headers, tc.bodies), 0o600); err != nil {
				t.Fatalf("write archive: %v", err)
			}
			err := ValidateRestoreArchive(path)
			if err == nil {
				t.Fatalf("the archive was accepted; members: %v", memberNames(tc.headers))
			}
			if !errors.Is(err, ErrInvalidBackupArchive) {
				t.Errorf("error = %v, want ErrInvalidBackupArchive", err)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}

// TestValidateRestoreArchive_AcceptsFilesThisAppWrites is the regression guard
// for the decision to drop the directory allowlist.
//
// `sysupgrade -b` archives the overlay upper layer, and this application itself
// writes /etc/crontabs/root (LED and WiFi schedules),
// /etc/dropbear/authorized_keys (POST /system/ssh-keys) and, after a password
// change, /etc/shadow. An allowlist covering only etc/config and etc/ppp
// therefore refused every genuine backup taken on this device. Authorization is
// the boundary for restore (ADR 0007 section 2): the endpoint requires a token,
// and an authenticated admin already holds root via POST /system/ssh-keys,
// firmware flash and factory reset. Enumerating paths defended nothing and broke
// the feature.
func TestValidateRestoreArchive_AcceptsFilesThisAppWrites(t *testing.T) {
	headers := []*tar.Header{
		{Name: "etc/", Typeflag: tar.TypeDir, Size: 0},
		{Name: "etc/config/network", Typeflag: tar.TypeReg, Size: 3},
		{Name: "etc/crontabs/root", Typeflag: tar.TypeReg, Size: 19},
		{Name: "etc/dropbear/authorized_keys", Typeflag: tar.TypeReg, Size: 3},
		{Name: "etc/shadow", Typeflag: tar.TypeReg, Size: 3},
		{Name: "etc/uci-defaults/99-payload", Typeflag: tar.TypeReg, Size: 3},
		{Name: "etc/init.d/", Typeflag: tar.TypeDir, Size: 0},
		{Name: "etc/ppp/chap-secrets", Typeflag: tar.TypeReg, Size: 3},
	}
	bodies := map[string]string{"etc/crontabs/root": "* * * * * /bin/sh -c id"}
	path := filepath.Join(t.TempDir(), "backup.tar.gz")
	if err := os.WriteFile(path, tarGzArchive(t, headers, bodies), 0o600); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	if err := ValidateRestoreArchive(path); err != nil {
		t.Errorf("a genuine overlay backup was refused: %v", err)
	}
}

func TestValidateRestoreArchive_RejectsNonGzip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.tar.gz")
	if err := os.WriteFile(path, []byte("not a gzip file at all"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := ValidateRestoreArchive(path); !errors.Is(err, ErrInvalidBackupArchive) {
		t.Errorf("error = %v, want ErrInvalidBackupArchive", err)
	}
	if err := ValidateRestoreArchive(filepath.Join(t.TempDir(), "missing.tar.gz")); !errors.Is(err, ErrInvalidBackupArchive) {
		t.Errorf("missing file: error = %v, want ErrInvalidBackupArchive", err)
	}
}

// A rejected archive must never reach sysupgrade — that call is what extracts
// it at / — and it must not leave a crash guard behind either: nothing was
// started, so a guard would tell the operator a restore is in flight.
func TestRestoreBackup_RejectsArchiveBeforeSysupgrade(t *testing.T) {
	svc, dir := newGuardedSystemService(t)
	marker := installFakeSysupgrade(t)

	path := filepath.Join(dir, "evil.tar.gz")
	evil := tarGzArchive(t, []*tar.Header{
		{Name: "etc/crontabs/root", Typeflag: tar.TypeReg, Size: 3},
	}, map[string]string{"etc/crontabs/root": "abc"})
	if err := os.WriteFile(path, evil, 0o600); err != nil {
		t.Fatalf("write archive: %v", err)
	}

	err := svc.RestoreBackup(path)
	if err == nil {
		t.Fatal("an archive writing outside the allowlist was restored")
	}
	if !errors.Is(err, ErrInvalidBackupArchive) {
		t.Errorf("error = %v, want ErrInvalidBackupArchive", err)
	}
	assertNoSysupgradeCall(t, marker)
	if _, err := os.Stat(guard.In(dir, restoreGuard).Path()); err == nil {
		t.Error("a rejected archive left a restore-in-progress guard behind")
	}
}

// The backup filename must be unique: the handler removes exactly the path it
// was handed, so two backups in the same second used to delete each other's
// archive while it was still streaming.
func TestUniqueTempPath_IsUniqueAndFree(t *testing.T) {
	dir := t.TempDir()
	seen := make(map[string]bool)
	for range 200 {
		p, err := uniqueTempPath(dir, "backup-*.tar.gz")
		if err != nil {
			t.Fatalf("uniqueTempPath: %v", err)
		}
		if seen[p] {
			t.Fatalf("duplicate temp path %q", p)
		}
		seen[p] = true
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("temp path %q must not exist yet, stat err = %v", p, err)
		}
		if !strings.HasPrefix(filepath.Base(p), "backup-") || !strings.HasSuffix(p, ".tar.gz") {
			t.Errorf("unexpected backup path shape: %q", p)
		}
	}
}

// A DIRECTORY entry under etc/config is not evidence of anything: the
// message claims a UCI config member, so the witness has to be a real file. An
// archive that carries only directories below etc/config (note that
// cleanRestoreName drops the trailing slash, so it is the subdirectories that
// match the prefix) would otherwise pass.
func TestValidateRestoreArchive_RejectsConfigDirectoryWithoutAFile(t *testing.T) {
	headers := []*tar.Header{
		{Name: "etc", Typeflag: tar.TypeDir, Mode: 0o755, Size: 0},
		{Name: "etc/config", Typeflag: tar.TypeDir, Mode: 0o755, Size: 0},
		{Name: "etc/config/network", Typeflag: tar.TypeDir, Mode: 0o755, Size: 0},
		{Name: "etc/config/dhcp/", Typeflag: tar.TypeDir, Mode: 0o755, Size: 0},
		{Name: "etc/shadow", Typeflag: tar.TypeReg, Mode: 0o600, Size: 3},
	}
	archive := tarGzArchive(t, headers, map[string]string{"etc/shadow": "abc"})
	path := filepath.Join(t.TempDir(), "empty-config.tar.gz")
	if err := os.WriteFile(path, archive, 0o600); err != nil {
		t.Fatalf("write archive: %v", err)
	}
	err := ValidateRestoreArchive(path)
	if err == nil {
		t.Fatal("an archive with only an etc/config directory was accepted as a backup")
	}
	if !errors.Is(err, ErrInvalidBackupArchive) {
		t.Errorf("error = %v, want ErrInvalidBackupArchive", err)
	}
	if !strings.Contains(err.Error(), "not a configuration backup") {
		t.Errorf("error = %q, want it to report the missing UCI config file", err)
	}
}
