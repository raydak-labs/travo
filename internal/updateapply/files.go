package updateapply

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/blackdark-org/travo-greenfield/internal/atomicfile"
)

const freeFloor = 20 << 20

// Owned by install.sh (docs/adr/0007): listed in the manifest, never replaced by an update.
var installerFiles = []string{
	"/etc/hotplug.d/button/50-travo-greenfield", "/etc/init.d/travo-greenfield",
	"/etc/uci-defaults/90-travo-greenfield", "/lib/upgrade/keep.d/travo-greenfield",
}

type layout struct {
	root, binary, ui, manifest, lock string
}

func realLayout() layout {
	return layout{"/etc/travo-greenfield/update-operation", "/usr/bin/travo-greenfield", "/www/travo-greenfield", "/etc/travo-greenfield/manifest.txt", "/var/lock/travo-greenfield-update.lock"}
}

func (l layout) path(parts ...string) string {
	return filepath.Join(append([]string{l.root}, parts...)...)
}

func syncDir(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func regular(path string) (os.FileInfo, error) {
	f, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !f.Mode().IsRegular() {
		return nil, fmt.Errorf("expected regular file: %s", path)
	}
	return f, nil
}

func tree(path string) ([]os.DirEntry, error) {
	st, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("expected directory: %s", path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if !e.Type().IsRegular() || strings.ContainsAny(e.Name(), "\r\n") || strings.Contains(e.Name(), "..") {
			return nil, fmt.Errorf("unexpected UI file: %s", e.Name())
		}
	}
	return entries, nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	if _, err := regular(src); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if err == nil {
		err = out.Sync()
	}
	return errors.Join(err, out.Close())
}

func copyTree(src, dst string) error {
	entries, err := tree(src)
	if err != nil {
		return err
	}
	if err := os.Mkdir(dst, 0755); err != nil {
		return err
	}
	for _, e := range entries {
		if err := copyFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()), 0644); err != nil {
			return err
		}
	}
	return syncDir(dst)
}

func uiPaths(prefix string, entries []os.DirEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, prefix+"/"+e.Name())
	}
	return out
}

func (l layout) replacementManifest(next []os.DirEntry) ([]byte, error) {
	current, err := tree(l.ui)
	if err != nil {
		return nil, err
	}
	if _, err = regular(l.manifest); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(l.manifest)
	if err != nil {
		return nil, err
	}
	allowed := append([]string{l.binary}, installerFiles...)
	expected := append(slices.Clone(allowed), uiPaths(l.ui, current)...)
	entries := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	slices.Sort(entries)
	slices.Sort(expected)
	if !slices.Equal(entries, expected) {
		return nil, errors.New("installed manifest does not match owned files")
	}
	result := append(allowed, uiPaths(l.ui, next)...)
	slices.Sort(result)
	return []byte(strings.Join(result, "\n") + "\n"), nil
}

type allocation struct {
	path  string
	bytes uint64
}

func fileBytes(path string) (uint64, error) {
	st, err := regular(path)
	if err != nil {
		return 0, err
	}
	// Charge one extra block per file to cover filesystem allocation rounding.
	return uint64(st.Size()) + 4096, nil
}

func treeBytes(path string) (uint64, error) {
	entries, err := tree(path)
	if err != nil {
		return 0, err
	}
	total := uint64(4096)
	for _, entry := range entries {
		bytes, err := fileBytes(filepath.Join(path, entry.Name()))
		if err != nil {
			return 0, err
		}
		total += bytes
	}
	return total, nil
}

func space(allocations []allocation) error {
	type volume struct{ free, needed uint64 }
	volumes := map[uint64]volume{}
	for _, a := range allocations {
		path := a.path
		st, err := os.Stat(path)
		for errors.Is(err, os.ErrNotExist) && path != filepath.Dir(path) {
			path = filepath.Dir(path)
			st, err = os.Stat(path)
		}
		if err != nil {
			return err
		}
		var fs syscall.Statfs_t
		if err := syscall.Statfs(path, &fs); err != nil {
			return err
		}
		dev := uint64(st.Sys().(*syscall.Stat_t).Dev)
		v := volumes[dev]
		v.free = uint64(fs.Bavail) * uint64(fs.Bsize)
		v.needed += a.bytes
		volumes[dev] = v
	}
	for _, v := range volumes {
		if v.free < freeFloor || v.free-freeFloor < v.needed {
			return errors.New("insufficient update space")
		}
	}
	return nil
}

func (l layout) prepare(payload, recovery string) error {
	incomingBin := filepath.Join(payload, "root/usr/bin/travo-greenfield")
	incomingUI := filepath.Join(payload, "root/www/travo-greenfield")
	next, err := tree(incomingUI)
	if err != nil {
		return err
	}
	manifest, err := l.replacementManifest(next)
	if err != nil {
		return err
	}
	oldBin, err := fileBytes(l.binary)
	if err != nil {
		return err
	}
	oldUI, err := treeBytes(l.ui)
	if err != nil {
		return err
	}
	newBin, err := fileBytes(incomingBin)
	if err != nil {
		return err
	}
	newUI, err := treeBytes(incomingUI)
	if err != nil {
		return err
	}
	rec, err := fileBytes(recovery)
	if err != nil {
		return err
	}
	oldManifest, err := fileBytes(l.manifest)
	if err != nil {
		return err
	}
	if err = space([]allocation{
		{l.root, oldBin + oldUI + oldManifest + newBin + newUI + rec + uint64(len(manifest)) + 65536},
		{filepath.Dir(l.binary), max(oldBin, newBin)},
		{filepath.Dir(l.ui), max(oldUI, newUI)},
		{filepath.Dir(l.manifest), max(oldManifest, uint64(len(manifest))+4096)},
	}); err != nil {
		return err
	}
	if err = os.MkdirAll(l.root, 0700); err != nil {
		return err
	}
	if err = syncDir(filepath.Dir(l.root)); err != nil {
		return err
	}
	if err = l.cleanScratch(); err != nil {
		return err
	}
	if err = os.Mkdir(l.path("next"), 0700); err != nil {
		return err
	}
	if err = os.Mkdir(l.path("rollback"), 0700); err != nil {
		return err
	}
	if err = copyFile(recovery, l.path("recovery"), 0700); err != nil {
		return err
	}
	if err = copyFile(incomingBin, l.path("next", "binary"), 0755); err != nil {
		return err
	}
	if err = copyTree(incomingUI, l.path("next", "ui")); err != nil {
		return err
	}
	if err = atomicfile.Write(l.path("next", "manifest"), manifest, 0600); err != nil {
		return err
	}
	return l.saveRollback()
}

func (l layout) saveRollback() error {
	if err := copyFile(l.binary, l.path("rollback", "binary"), 0755); err != nil {
		return err
	}
	if err := copyTree(l.ui, l.path("rollback", "ui")); err != nil {
		return err
	}
	if err := copyFile(l.manifest, l.path("rollback", "manifest"), 0600); err != nil {
		return err
	}
	if err := syncDir(l.path("rollback")); err != nil {
		return err
	}
	return syncDir(l.root)
}

func (l layout) cleanScratch() error {
	for _, name := range []string{"next", "rollback", "recovery"} {
		if err := os.RemoveAll(l.path(name)); err != nil {
			return err
		}
	}
	return syncDir(l.root)
}

func installFile(src, dst string, mode os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return atomicfile.Write(dst, data, mode)
}

func (l layout) installAssets(slot string) error {
	if err := installFile(l.path(slot, "binary"), l.binary, 0755); err != nil {
		return err
	}
	// The service is stopped. The durable journal covers the interval without a UI tree.
	tmp := l.ui + ".update-next"
	if err := os.RemoveAll(tmp); err != nil {
		return err
	}
	if err := copyTree(l.path(slot, "ui"), tmp); err != nil {
		return err
	}
	if err := os.RemoveAll(l.ui); err != nil {
		return err
	}
	if err := os.Rename(tmp, l.ui); err != nil {
		return err
	}
	if err := syncDir(filepath.Dir(l.ui)); err != nil {
		return err
	}
	return installFile(l.path(slot, "manifest"), l.manifest, 0600)
}

func (l layout) promote() error {
	if _, err := os.Lstat(l.path("rollback")); err == nil {
		if _, err := os.Lstat(l.path(".old")); err == nil {
			if err := os.Rename(l.path(".old"), l.path(".old-retired")); err != nil {
				return err
			}
			if err := syncDir(l.root); err != nil {
				return err
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.Rename(l.path("rollback"), l.path(".old")); err != nil {
			return err
		}
		if err := syncDir(l.root); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if _, err := regular(l.path(".old", "binary")); err != nil {
		return err
	}
	if _, err := tree(l.path(".old", "ui")); err != nil {
		return err
	}
	if _, err := regular(l.path(".old", "manifest")); err != nil {
		return err
	}
	if err := os.RemoveAll(l.path(".old-retired")); err != nil {
		return err
	}
	return syncDir(l.root)
}
