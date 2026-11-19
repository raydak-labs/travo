package pkgmgr

import (
	"context"
	"fmt"
	"strings"
)

// Upgrade is an installed package with a newer version in the package lists.
type Upgrade struct {
	Name      string `json:"name"`
	Installed string `json:"installed"`
	Available string `json:"available"`
}

// UpgradeLister reads upgradable packages from the lists already in /tmp. It never
// refreshes them (Update does) and never installs anything.
type UpgradeLister interface {
	Upgradable(ctx context.Context) ([]Upgrade, error)
}

// Upgradable runs `apk list --upgradable`.
func (a *APK) Upgradable(ctx context.Context) ([]Upgrade, error) {
	out, _, err := run(ctx, a.Run, TierQuery, "apk", "list", "--upgradable")
	if err != nil {
		return nil, err
	}
	return ParseAPKUpgradable(out)
}

// Upgradable runs `opkg list-upgradable`.
func (o *OPKG) Upgradable(ctx context.Context) ([]Upgrade, error) {
	out, _, err := run(ctx, o.Run, TierQuery, "opkg", "list-upgradable")
	if err != nil {
		return nil, err
	}
	return ParseOPKGUpgradable(out)
}

// Upgradable reports nothing to upgrade in mock mode.
func (*Mock) Upgradable(context.Context) ([]Upgrade, error) { return []Upgrade{}, nil }

// ParseAPKUpgradable parses apk-tools 3 lines:
// `<name>-<new> <arch> {<origin>} (<license>) [upgradable from: <name>-<old>]`.
func ParseAPKUpgradable(out string) ([]Upgrade, error) {
	ups := []Upgrade{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		head, from, ok := strings.Cut(line, " [upgradable from: ")
		fields := strings.Fields(head)
		if !ok || len(fields) == 0 || !strings.HasSuffix(from, "]") {
			return nil, fmt.Errorf("apk list --upgradable: malformed line %q", line)
		}
		name, available, ok1 := splitAPKName(fields[0])
		oldName, installed, ok2 := splitAPKName(strings.TrimSuffix(from, "]"))
		if !ok1 || !ok2 || name != oldName {
			return nil, fmt.Errorf("apk list --upgradable: malformed line %q", line)
		}
		ups = append(ups, Upgrade{Name: name, Installed: installed, Available: available})
	}
	return ups, nil
}

// splitAPKName splits `name-version` at the first '-' followed by a digit, the apk
// convention (package names may contain '-', versions start with a digit).
func splitAPKName(s string) (name, version string, ok bool) {
	for i := 1; i+1 < len(s); i++ {
		if s[i] == '-' && s[i+1] >= '0' && s[i+1] <= '9' {
			return s[:i], s[i+1:], true
		}
	}
	return "", "", false
}

// ParseOPKGUpgradable parses `name - installed - available` lines.
func ParseOPKGUpgradable(out string) ([]Upgrade, error) {
	ups := []Upgrade{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		f := strings.Split(line, " - ")
		if len(f) != 3 || f[0] == "" || f[1] == "" || f[2] == "" {
			return nil, fmt.Errorf("opkg list-upgradable: malformed line %q", line)
		}
		ups = append(ups, Upgrade{Name: f[0], Installed: f[1], Available: f[2]})
	}
	return ups, nil
}

var (
	_ UpgradeLister = (*APK)(nil)
	_ UpgradeLister = (*OPKG)(nil)
	_ UpgradeLister = (*Mock)(nil)
)
