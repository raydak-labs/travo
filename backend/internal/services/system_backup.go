package services

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/execx"
)

// CreateBackup generates a configuration backup archive and returns its path.
// The path is unique per call: the handler removes exactly what it created, so
// two backups started in the same second must not share a filename.
func (s *SystemService) CreateBackup() (string, error) {
	path, err := uniqueTempPath("/tmp", "backup-*.tar.gz")
	if err != nil {
		return "", err
	}
	out, err := execx.CombinedOutput(execx.Slow, "sysupgrade", "-b", path)
	if err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("creating backup: %w: %s", err, string(out))
	}
	return path, nil
}

// uniqueTempPath reserves a name that does not yet exist and returns it.
func uniqueTempPath(dir, pattern string) (string, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", fmt.Errorf("creating temp path: %w", err)
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		return "", err
	}
	// sysupgrade -b wants to create the file itself.
	if err := os.Remove(name); err != nil {
		return "", err
	}
	return name, nil
}

// RestoreBackup applies a configuration backup from the given file path.
//
// The archive is validated FIRST: `sysupgrade -r` extracts the tarball at /,
// so an unvalidated upload is an arbitrary-root-file-write primitive (any path
// under /etc/crontabs, /etc/init.d or /etc/uci-defaults becomes a root code
// execution path). Validation happens before the crash guard is written because
// a rejected archive has touched nothing, so it must leave no state behind.
//
// A guard is then written and only removed once sysupgrade returned
// successfully: `sysupgrade -r` rewrites /etc/config (ADR 0003).
func (s *SystemService) RestoreBackup(path string) error {
	if err := ValidateRestoreArchive(path); err != nil {
		return err
	}
	if err := s.writeGuard(restoreGuardName, "restore backup "+path); err != nil {
		return err
	}
	out, err := execx.CombinedOutput(execx.Slow, "sysupgrade", "-r", path)
	if err != nil {
		// Guard stays: the restore may have left /etc/config half-written.
		return fmt.Errorf("restoring backup (guard %s kept): %w: %s", restoreGuardName, err, string(out))
	}
	s.clearGuard(restoreGuardName)
	return nil
}

// ValidateRestoreArchive checks that path is a structurally safe gzip tar
// archive carrying at least one UCI config member.
//
// Rejected: absolute paths, ".." traversal, symlinks, hardlinks, device nodes,
// archives exceeding the size/member caps, and archives with nothing under
// etc/config (i.e. not a configuration backup).
//
// It does NOT police WHICH paths an administrator may write. Authorization is
// that boundary (ADR 0007 section 2), and this function exists to reject
// malformed and hostile archives — traversal, link substitution and
// decompression bombs — so a corrupted or third-party file cannot damage a
// device the operator is trying to recover.
func ValidateRestoreArchive(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidBackupArchive, err)
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("%w: not a gzip archive: %w", ErrInvalidBackupArchive, err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	var total int64
	members := 0
	sawUCIConfig := false
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("%w: unreadable tar stream: %w", ErrInvalidBackupArchive, err)
		}
		members++
		if members > maxRestoreArchiveMembers {
			return fmt.Errorf("%w: more than %d members", ErrInvalidBackupArchive, maxRestoreArchiveMembers)
		}
		if err := checkRestoreMember(hdr); err != nil {
			return err
		}
		if hdr.Size < 0 || hdr.Size > maxRestoreMemberBytes {
			return fmt.Errorf("%w: member %q is larger than %d bytes", ErrInvalidBackupArchive, hdr.Name, maxRestoreMemberBytes)
		}
		total += hdr.Size
		if total > maxRestoreArchiveBytes {
			return fmt.Errorf("%w: uncompressed size exceeds %d bytes", ErrInvalidBackupArchive, maxRestoreArchiveBytes)
		}
		// A DIRECTORY entry under etc/config is not evidence of anything: the
		// message claims a UCI config member, so the witness has to be a real
		// file. An archive carrying only "etc/config/" would otherwise pass.
		if hdr.Typeflag == tar.TypeReg &&
			strings.HasPrefix(cleanRestoreName(hdr.Name), restoreArchiveNeedsUCIConfig) {
			sawUCIConfig = true
		}
	}
	if members == 0 {
		return fmt.Errorf("%w: archive is empty", ErrInvalidBackupArchive)
	}
	if !sawUCIConfig {
		return fmt.Errorf("%w: not a configuration backup: no member under %s",
			ErrInvalidBackupArchive, restoreArchiveNeedsUCIConfig)
	}
	return nil
}

// cleanRestoreName normalises a member path for the etc/config membership test.
func cleanRestoreName(name string) string {
	return path.Clean(strings.TrimPrefix(name, "/"))
}

// checkRestoreMember rejects one tar header that sysupgrade -r would extract
// unsafely — outside the archive root, or as anything other than a plain file.
func checkRestoreMember(hdr *tar.Header) error {
	name := hdr.Name
	if name == "" {
		return fmt.Errorf("%w: member with an empty name", ErrInvalidBackupArchive)
	}
	// sysupgrade -r extracts relative to /, so a leading slash writes outside
	// the config tree and "..", however it is spelled, walks out of it.
	if strings.HasPrefix(name, "/") || strings.Contains(name, `\`) {
		return fmt.Errorf("%w: member %q is not a relative path", ErrInvalidBackupArchive, name)
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("%w: member %q escapes the archive root", ErrInvalidBackupArchive, name)
	}
	// Only regular files and the directories that contain them: a symlink or
	// hard link member is a write-what-where primitive (tar replaces the link
	// target's contents, or re-points it), and device nodes have no business in
	// a config backup.
	switch hdr.Typeflag {
	case tar.TypeReg, tar.TypeDir:
	default:
		return fmt.Errorf("%w: member %q has type %q, only regular files are allowed",
			ErrInvalidBackupArchive, name, string(rune(hdr.Typeflag)))
	}
	return nil
}
