package pkgmgr

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
)

// Mock is an in-memory PackageManager for mock mode and tests.
type Mock struct {
	mu   sync.Mutex
	pkgs map[string]string
	// Files are the paths InstallFile was called with.
	Files []string
}

func NewMock() *Mock {
	return &Mock{pkgs: map[string]string{"dnsmasq": "2.93-r1", "dropbear": "2025.89-r1"}}
}

func (*Mock) Name() string                 { return "mock" }
func (*Mock) Update(context.Context) error { return nil }

func (m *Mock) Install(_ context.Context, pkg string) error {
	if err := validPkg(pkg); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pkgs[pkg] = "1.0-r1"
	return nil
}

// InstallFile installs the package named by the file's base name up to the first '-'
// followed by a digit (adguardhome-0.107.80-r1.apk -> adguardhome); Files records the call.
func (m *Mock) InstallFile(_ context.Context, path string) error {
	if err := validFile(path); err != nil {
		return err
	}
	name := filepath.Base(path)
	for i := 0; i+1 < len(name); i++ {
		if name[i] == '-' && name[i+1] >= '0' && name[i+1] <= '9' {
			name = name[:i]
			break
		}
	}
	name = strings.TrimSuffix(strings.TrimSuffix(name, ".apk"), ".ipk")
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pkgs[name] = "1.0-r1"
	m.Files = append(m.Files, path)
	return nil
}

func (m *Mock) Remove(_ context.Context, pkg string) error {
	if err := validPkg(pkg); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.pkgs, pkg)
	return nil
}

func (m *Mock) Installed(context.Context) ([]Package, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Package, 0, len(m.pkgs))
	for n, v := range m.pkgs {
		out = append(out, Package{Name: n, Version: v})
	}
	return out, nil
}

func (m *Mock) Version(ctx context.Context, pkg string) (string, error) {
	pkgs, _ := m.Installed(ctx)
	return find(pkgs, pkg)
}

var _ PackageManager = (*Mock)(nil)
var _ PackageManager = (*APK)(nil)
var _ PackageManager = (*OPKG)(nil)
