package updatestage

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

func privateOwner(info os.FileInfo, mode os.FileMode) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(stat.Uid) == os.Geteuid() && info.Mode().Perm() == mode
}

func lockRoot(root string) (*os.File, error) {
	if err := os.Mkdir(root, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || !privateOwner(info, 0700) {
		return nil, errors.New("staging root is not a private directory")
	}
	lock, err := os.OpenFile(filepath.Join(root, "lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return nil, err
	}
	info, err = lock.Stat()
	if err != nil || !info.Mode().IsRegular() || !privateOwner(info, 0600) {
		lock.Close()
		return nil, errors.New("invalid staging lock")
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, err
	}
	return lock, nil
}

func cleanStale(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !stageName.MatchString(entry.Name()) || !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if privateOwner(info, 0700) {
			if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
