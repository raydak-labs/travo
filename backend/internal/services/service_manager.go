package services

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/openwrt-travel-gui/backend/internal/execx"
	"github.com/openwrt-travel-gui/backend/internal/models"
)

// PackageManager abstracts package install/remove operations.
type PackageManager interface {
	// Update refreshes the package index. On OpenWrt the opkg lists live in
	// /tmp (RAM), so after every reboot an install without a prior update
	// fails — callers run this best-effort before installing.
	Update() (string, error)
	Install(pkg string) (string, error)
	Remove(pkg string) (string, error)
	IsInstalled(pkg string) bool
	InstallStream(pkg string, logFn func(string)) error
	RemoveStream(pkg string, logFn func(string)) error
}

// SystemProbe abstracts init.d and process checks.
type SystemProbe interface {
	HasInitScript(name string) bool
	IsRunning(initName string) bool
	Start(initName string) (string, error)
	Stop(initName string) (string, error)
	IsAutoStart(initName string) bool
	Enable(initName string) error
	Disable(initName string) error
}

// serviceDefinition holds static config for a known service.
type serviceDefinition struct {
	ID            string
	Name          string
	Description   string
	Packages      []string // apk/opkg packages to install
	DetectPackage string   // primary package for installed detection (defaults to Packages[0])
	InitName      string   // init.d script name (empty if no init script)
}

var knownServices = []serviceDefinition{
	{
		ID: "adguardhome", Name: "AdGuard Home",
		Description: "Network-wide ad and tracker blocking DNS server",
		Packages:    []string{"adguardhome"},
		InitName:    "adguardhome",
	},
	{
		ID: "wireguard", Name: "WireGuard",
		Description:   "Fast, modern VPN tunnel",
		Packages:      []string{"wireguard-tools", "kmod-wireguard", "luci-proto-wireguard"},
		DetectPackage: "wireguard-tools", // kmod-wireguard may be built-in; detect by userspace tools
		InitName:      "",                // managed via UCI/netifd, no init.d
	},
	{
		ID: "tailscale", Name: "Tailscale",
		Description: "Zero-config mesh VPN",
		Packages:    []string{"tailscale"},
		InitName:    "tailscale",
	},
	{
		ID:          "vnstat",
		Name:        "Data Usage (vnstat)",
		Description: "Lightweight network traffic monitor with persistent counters",
		Packages:    []string{"vnstat2"},
		InitName:    "vnstat",
	},
	{
		ID:            "sqm",
		Name:          "SQM (Traffic Shaping)",
		Description:   "Smart Queue Management to reduce latency (bufferbloat)",
		Packages:      []string{"sqm-scripts", "luci-app-sqm"},
		DetectPackage: "sqm-scripts",
		InitName:      "sqm",
	},
	{
		ID:          "mwan3",
		Name:        "Connection Failover (mwan3)",
		Description: "Ordered multi-WAN failover with health tracking",
		Packages:    []string{"mwan3"},
		InitName:    "mwan3",
	},
	{
		ID:          "watchcat",
		Name:        "Watchcat",
		Description: "Connection watchdog — auto-reboot or restart interfaces on connectivity loss",
		Packages:    []string{"watchcat"},
		InitName:    "watchcat",
	},
	{
		ID:          "cloudflared",
		Name:        "Cloudflare Tunnel",
		Description: "Expose local services securely via Cloudflare network without port forwarding",
		Packages:    []string{"cloudflared"},
		InitName:    "cloudflared",
	},
}

// ServiceManager manages installable services.
type ServiceManager struct {
	// mu guards the cache snapshot only. It is deliberately NOT held across
	// package-manager or init.d calls: those run for up to execx.Package
	// (10 min) per package, and holding the write lock blocked
	// ListServices/GetServiceStatus for the whole install.
	mu    sync.RWMutex
	defs  []serviceDefinition
	pkg   PackageManager
	probe SystemProbe
	cache map[string]models.ServiceInfo
	// opMu serializes mutating operations (install/remove/start/stop) against
	// each other while leaving reads lock-free.
	opMu             sync.Mutex
	postInstallHooks map[string]func() error
	// guardDir holds the crash-guard file for package installs (ADR 0003);
	// empty means crashGuardDir (/etc/trafo). Tests point it at a temp dir.
	guardDir string
}

// SetPostInstallHook registers a callback that runs after successful package install
// for the given service ID. Useful for auto-configuration (e.g. AdGuard Home).
func (sm *ServiceManager) SetPostInstallHook(serviceID string, hook func() error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.postInstallHooks == nil {
		sm.postInstallHooks = make(map[string]func() error)
	}
	sm.postInstallHooks[serviceID] = hook
}

// NewServiceManager creates a ServiceManager that detects real system state.
func NewServiceManager() *ServiceManager {
	sm := NewServiceManagerWith(detectPackageManager(), &RealSystemProbe{})
	sm.RefreshCache()
	return sm
}

// NewServiceManagerWith creates a ServiceManager with injected dependencies (for tests).
func NewServiceManagerWith(pkg PackageManager, probe SystemProbe) *ServiceManager {
	return &ServiceManager{
		defs:  knownServices,
		pkg:   pkg,
		probe: probe,
		cache: make(map[string]models.ServiceInfo),
	}
}

// RefreshCache reloads the state of all services from the system.
func (sm *ServiceManager) RefreshCache() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for _, def := range sm.defs {
		sm.cache[def.ID] = sm.buildInfo(def)
	}
}

// refreshOne recomputes and publishes the cached state for a single service.
//
// The live-state probe runs OUTSIDE the lock (it shells out to opkg/init.d),
// and only the map write is serialized by the write lock. Two properties this
// must keep:
//   - sm.cache is never written without the write lock, while ListServices and
//     GetServiceStatus read it under the read lock. A lock-free write here is
//     a Go map read/write data race, which aborts the whole process.
//   - the write lock is not held across the probe, so a long-running
//     pkg.Update()/Install() (execx.Package = 10 min) cannot block readers.
func (sm *ServiceManager) refreshOne(serviceID string) {
	def, err := sm.findDef(serviceID)
	if err != nil {
		return
	}
	info := sm.buildInfo(def)
	sm.mu.Lock()
	sm.cache[def.ID] = info
	sm.mu.Unlock()
}

// ListServices returns all known services from cache.
func (sm *ServiceManager) ListServices() ([]models.ServiceInfo, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	result := make([]models.ServiceInfo, 0, len(sm.defs))
	for _, def := range sm.defs {
		if info, ok := sm.cache[def.ID]; ok {
			result = append(result, info)
		} else {
			result = append(result, sm.buildInfo(def))
		}
	}
	return result, nil
}

// GetServiceStatus returns the status of a specific service from cache.
func (sm *ServiceManager) GetServiceStatus(serviceID string) (models.ServiceInfo, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if info, ok := sm.cache[serviceID]; ok {
		return info, nil
	}
	for _, def := range sm.defs {
		if def.ID == serviceID {
			return sm.buildInfo(def), nil
		}
	}
	return models.ServiceInfo{}, fmt.Errorf("service not found: %s", serviceID)
}

// buildInfo checks live system state for a service definition.
func (sm *ServiceManager) buildInfo(def serviceDefinition) models.ServiceInfo {
	info := models.ServiceInfo{
		ID:          def.ID,
		Name:        def.Name,
		Description: def.Description,
		State:       "not_installed",
	}

	installed := true
	detect := def.Packages
	if def.DetectPackage != "" {
		detect = []string{def.DetectPackage}
	}
	for _, pkg := range detect {
		if !sm.pkg.IsInstalled(pkg) {
			installed = false
			break
		}
	}
	if !installed {
		return info
	}

	info.State = "stopped"
	if def.InitName != "" {
		info.AutoStart = sm.probe.IsAutoStart(def.InitName)
		if sm.probe.IsRunning(def.InitName) {
			info.State = "running"
		}
	} else {
		// Services without init.d (like wireguard via UCI) are "installed" when package present
		info.State = "installed"
	}
	return info
}

// SetGuardDir overrides the directory used for crash-guard files (tests).
func (sm *ServiceManager) SetGuardDir(dir string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.guardDir = dir
}

// pkgInstallGuardFile is the crash guard for package installs. Installing a
// package rewrites init scripts, /etc/config and kernel modules, so a power cut
// mid-install leaves a half-configured system with no marker (ADR 0003).
func (sm *ServiceManager) guardPath() string {
	sm.mu.RLock()
	dir := sm.guardDir
	sm.mu.RUnlock()
	if dir == "" {
		dir = crashGuardDir
	}
	return filepath.Join(dir, "pkg-install-in-progress")
}

// writeInstallGuard records that a mutating package operation has started.
//
// It REFUSES when the guard already exists (ADR 0003 §1.2): a guard on disk
// means a previous install or remove died part-way — init scripts rewritten,
// the package database mid-update — and blindly retrying it is precisely what
// the guard exists to prevent. The recovery path is deploy-local.sh or removing
// the file by hand, both of which clear every guard listed in ADR 0003 §2.
func (sm *ServiceManager) writeInstallGuard(reason string) error {
	path := sm.guardPath()
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("pkg guard: %s exists from an interrupted install/remove; "+
			"remove it or redeploy before retrying", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("pkg guard: stat %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("pkg guard: mkdir: %w", err)
	}
	if err := os.WriteFile(path, []byte(reason+"\n"), 0600); err != nil {
		return fmt.Errorf("pkg guard: write: %w", err)
	}
	return nil
}

// clearInstallGuard removes the guard after a fully successful operation.
// clearInstallGuard removes the package-install crash guard. Best-effort by
// design: the operation has already succeeded, and a stale guard only makes the
// next install wait for a redeploy, so the removal error is deliberately dropped.
func (sm *ServiceManager) clearInstallGuard() {
	_ = os.Remove(sm.guardPath())
}

// findDefUnlocked scans the immutable service catalog. sm.defs is assigned once
// in the constructor and never mutated, so reading it needs no lock; the lock in
// findDef exists to keep the pair safe to call from anywhere.
func (sm *ServiceManager) findDefUnlocked(serviceID string) (serviceDefinition, bool) {
	for _, def := range sm.defs {
		if def.ID == serviceID {
			return def, true
		}
	}
	return serviceDefinition{}, false
}

// findDef resolves a service ID to its definition. defs are immutable after
// construction, so this needs no lock and can safely run outside the write
// lock.
func (sm *ServiceManager) findDef(serviceID string) (serviceDefinition, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if def, ok := sm.findDefUnlocked(serviceID); ok {
		return def, nil
	}
	return serviceDefinition{}, fmt.Errorf("service not found: %s", serviceID)
}

// postInstallHook returns the registered hook for a service, if any.
func (sm *ServiceManager) postInstallHook(serviceID string) func() error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.postInstallHooks[serviceID]
}

// Install installs the packages for a service.
func (sm *ServiceManager) Install(serviceID string) error {
	// opMu serializes mutating work; the cache write lock is only taken for the
	// short refreshOne call at the end, so reads stay responsive while a
	// 3-package install runs (previously 30 minutes of blocking).
	sm.opMu.Lock()
	defer sm.opMu.Unlock()

	def, err := sm.findDef(serviceID)
	if err != nil {
		return err
	}
	if err := sm.writeInstallGuard("install " + serviceID); err != nil {
		return err
	}
	// Best-effort index refresh: a failure (e.g. offline) still leaves the
	// install attempt as the authoritative error.
	_, _ = sm.pkg.Update()
	for _, pkg := range def.Packages {
		if out, err := sm.pkg.Install(pkg); err != nil {
			// Guard stays: the system may be half-installed and an operator
			// needs the marker (ADR 0003).
			return fmt.Errorf("failed to install %s: %w\n%s", pkg, err, out)
		}
	}
	sm.refreshOne(serviceID)
	if hook := sm.postInstallHook(serviceID); hook != nil {
		// The hook configures the freshly installed service (e.g. AdGuard
		// Home). Swallowing its error made the API report success for a service
		// that is installed but unconfigured.
		if err := hook(); err != nil {
			return fmt.Errorf("post-install configuration for %s failed: %w", serviceID, err)
		}
	}
	sm.clearInstallGuard()
	return nil
}

// Remove removes the packages for a service.
func (sm *ServiceManager) Remove(serviceID string) error {
	sm.opMu.Lock()
	defer sm.opMu.Unlock()

	def, err := sm.findDef(serviceID)
	if err != nil {
		return err
	}
	if err := sm.writeInstallGuard("remove " + serviceID); err != nil {
		return err
	}
	// Stop first if running
	if def.InitName != "" && sm.probe.IsRunning(def.InitName) {
		_, _ = sm.probe.Stop(def.InitName)
	}
	for _, pkg := range def.Packages {
		if out, err := sm.pkg.Remove(pkg); err != nil {
			return fmt.Errorf("failed to remove %s: %w\n%s", pkg, err, out)
		}
	}
	sm.refreshOne(serviceID)
	sm.clearInstallGuard()
	return nil
}

// InstallWithLog installs packages and streams output line by line via logFn.
func (sm *ServiceManager) InstallWithLog(serviceID string, logFn func(string)) error {
	sm.opMu.Lock()
	defer sm.opMu.Unlock()

	def, err := sm.findDef(serviceID)
	if err != nil {
		return err
	}
	if err := sm.writeInstallGuard("install " + serviceID); err != nil {
		logFn(fmt.Sprintf("Cannot start: %v", err))
		return err
	}
	logFn("Updating package index…")
	if _, err := sm.pkg.Update(); err != nil {
		logFn(fmt.Sprintf("Index update warning (continuing): %s", err.Error()))
	}
	for _, pkg := range def.Packages {
		logFn(fmt.Sprintf("Installing package: %s", pkg))
		if err := sm.pkg.InstallStream(pkg, logFn); err != nil {
			return fmt.Errorf("failed to install %s: %w", pkg, err)
		}
	}
	sm.refreshOne(serviceID)
	if hook := sm.postInstallHook(serviceID); hook != nil {
		logFn("Running post-install configuration…")
		if err := hook(); err != nil {
			return fmt.Errorf("post-install configuration for %s failed: %w", serviceID, err)
		}
		logFn("Post-install configuration complete.")
	}
	sm.clearInstallGuard()
	return nil
}

// RemoveWithLog removes packages and streams output line by line via logFn.
func (sm *ServiceManager) RemoveWithLog(serviceID string, logFn func(string)) error {
	sm.opMu.Lock()
	defer sm.opMu.Unlock()

	def, err := sm.findDef(serviceID)
	if err != nil {
		return err
	}
	if err := sm.writeInstallGuard("remove " + serviceID); err != nil {
		logFn(fmt.Sprintf("Cannot start: %v", err))
		return err
	}
	// Stop first if running
	if def.InitName != "" && sm.probe.IsRunning(def.InitName) {
		logFn(fmt.Sprintf("Stopping %s...", def.InitName))
		_, _ = sm.probe.Stop(def.InitName)
	}
	for _, pkg := range def.Packages {
		logFn(fmt.Sprintf("Removing package: %s", pkg))
		if err := sm.pkg.RemoveStream(pkg, logFn); err != nil {
			return fmt.Errorf("failed to remove %s: %w", pkg, err)
		}
	}
	sm.refreshOne(serviceID)
	sm.clearInstallGuard()
	return nil
}

// Start starts a service via init.d.
//
// No crash guard, deliberately: every service in the catalog is an ordinary
// procd daemon (adguardhome, tailscale, vnstat, sqm, mwan3, watchcat,
// cloudflared), and `init.d <name> start` is idempotent and retryable — procd
// converges on "running" however often it is repeated, and a repeat after a
// power cut cannot make the outcome worse. A guard here would satisfy
// ADR 0003 §1 only formally: it would disable start/stop for the whole device
// after one power cut until a redeploy, and ADR 0003 §2 lists what a guard has
// to protect. Package install/remove, which is NOT retryable, is guarded above.
func (sm *ServiceManager) Start(serviceID string) error {
	sm.opMu.Lock()
	defer sm.opMu.Unlock()

	def, err := sm.findDef(serviceID)
	if err != nil {
		return err
	}
	if def.InitName == "" {
		return fmt.Errorf("service %s does not have an init script", serviceID)
	}
	if !sm.probe.HasInitScript(def.InitName) {
		return fmt.Errorf("service %s not installed", serviceID)
	}
	out, err := sm.probe.Start(def.InitName)
	if err != nil {
		return fmt.Errorf("failed to start %s: %w\n%s", serviceID, err, out)
	}
	sm.refreshOne(serviceID)
	return nil
}

// Stop stops a service via init.d. Unguarded for the same reason as Start.
func (sm *ServiceManager) Stop(serviceID string) error {
	sm.opMu.Lock()
	defer sm.opMu.Unlock()

	def, err := sm.findDef(serviceID)
	if err != nil {
		return err
	}
	if def.InitName == "" {
		return fmt.Errorf("service %s does not have an init script", serviceID)
	}
	out, err := sm.probe.Stop(def.InitName)
	if err != nil {
		return fmt.Errorf("failed to stop %s: %w\n%s", serviceID, err, out)
	}
	sm.refreshOne(serviceID)
	return nil
}

// SetAutoStart enables or disables auto-start for a service.
func (sm *ServiceManager) SetAutoStart(serviceID string, enabled bool) error {
	sm.opMu.Lock()
	defer sm.opMu.Unlock()

	def, err := sm.findDef(serviceID)
	if err != nil {
		return err
	}
	if def.InitName == "" {
		return fmt.Errorf("service %s does not have an init script", serviceID)
	}
	sm.mu.RLock()
	info, ok := sm.cache[def.ID]
	sm.mu.RUnlock()
	if !ok || info.State == "not_installed" {
		return fmt.Errorf("service %s is not installed", serviceID)
	}
	if enabled {
		if err := sm.probe.Enable(def.InitName); err != nil {
			return fmt.Errorf("enabling auto-start for %s: %w", def.InitName, err)
		}
	} else {
		if err := sm.probe.Disable(def.InitName); err != nil {
			return fmt.Errorf("disabling auto-start for %s: %w", def.InitName, err)
		}
	}
	sm.refreshOne(serviceID)
	return nil
}

// --- Real implementations ---

// detectPackageManager checks which package manager is available.
func detectPackageManager() PackageManager {
	if _, err := exec.LookPath("apk"); err == nil {
		return &ApkPackageManager{}
	}
	if _, err := exec.LookPath("opkg"); err == nil {
		return &OpkgPackageManager{}
	}
	return &NoopPackageManager{}
}

// ApkPackageManager uses apk (OpenWrt 25.x+).
type ApkPackageManager struct{}

func (a *ApkPackageManager) Update() (string, error) {
	out, err := execx.CombinedOutput(execx.Package, "apk", "update")
	return string(out), err
}
func (a *ApkPackageManager) Install(pkg string) (string, error) {
	out, err := execx.CombinedOutput(execx.Package, "apk", "add", pkg)
	return string(out), err
}
func (a *ApkPackageManager) Remove(pkg string) (string, error) {
	out, err := execx.CombinedOutput(execx.Package, "apk", "del", pkg)
	return string(out), err
}
func (a *ApkPackageManager) IsInstalled(pkg string) bool {
	err := execx.Run(execx.Quick, "apk", "info", "-e", pkg)
	return err == nil
}
func (a *ApkPackageManager) InstallStream(pkg string, logFn func(string)) error {
	return execx.Stream(execx.Package, logFn, "apk", "add", pkg)
}
func (a *ApkPackageManager) RemoveStream(pkg string, logFn func(string)) error {
	return execx.Stream(execx.Package, logFn, "apk", "del", pkg)
}

// OpkgPackageManager uses opkg (OpenWrt <25).
type OpkgPackageManager struct{}

func (o *OpkgPackageManager) Update() (string, error) {
	out, err := execx.CombinedOutput(execx.Package, "opkg", "update")
	return string(out), err
}
func (o *OpkgPackageManager) Install(pkg string) (string, error) {
	out, err := execx.CombinedOutput(execx.Package, "opkg", "install", pkg)
	return string(out), err
}
func (o *OpkgPackageManager) Remove(pkg string) (string, error) {
	out, err := execx.CombinedOutput(execx.Package, "opkg", "remove", pkg)
	return string(out), err
}
func (o *OpkgPackageManager) IsInstalled(pkg string) bool {
	out, err := execx.CombinedOutput(execx.Quick, "opkg", "list-installed", pkg)
	return err == nil && strings.Contains(string(out), pkg)
}
func (o *OpkgPackageManager) InstallStream(pkg string, logFn func(string)) error {
	return execx.Stream(execx.Package, logFn, "opkg", "install", pkg)
}
func (o *OpkgPackageManager) RemoveStream(pkg string, logFn func(string)) error {
	return execx.Stream(execx.Package, logFn, "opkg", "remove", pkg)
}

// NoopPackageManager for systems without a package manager.
type NoopPackageManager struct{}

func (n *NoopPackageManager) Update() (string, error) {
	return "", fmt.Errorf("no package manager available")
}
func (n *NoopPackageManager) Install(string) (string, error) {
	return "", fmt.Errorf("no package manager available")
}
func (n *NoopPackageManager) Remove(string) (string, error) {
	return "", fmt.Errorf("no package manager available")
}
func (n *NoopPackageManager) IsInstalled(string) bool { return false }
func (n *NoopPackageManager) InstallStream(string, func(string)) error {
	return fmt.Errorf("no package manager available")
}
func (n *NoopPackageManager) RemoveStream(string, func(string)) error {
	return fmt.Errorf("no package manager available")
}

// RealSystemProbe checks init.d scripts and running processes.
type RealSystemProbe struct{}

func (r *RealSystemProbe) HasInitScript(name string) bool {
	_, err := os.Stat("/etc/init.d/" + name)
	return err == nil
}
func (r *RealSystemProbe) IsRunning(initName string) bool {
	// OpenWrt init.d scripts return 0 for "running" status
	err := execx.Run(execx.Quick, "/etc/init.d/"+initName, "status")
	return err == nil
}
func (r *RealSystemProbe) Start(initName string) (string, error) {
	out, err := execx.CombinedOutput(execx.Slow, "/etc/init.d/"+initName, "start")
	return string(out), err
}
func (r *RealSystemProbe) Stop(initName string) (string, error) {
	out, err := execx.CombinedOutput(execx.Slow, "/etc/init.d/"+initName, "stop")
	return string(out), err
}
func (r *RealSystemProbe) IsAutoStart(initName string) bool {
	err := execx.Run(execx.Quick, "/etc/init.d/"+initName, "enabled")
	return err == nil
}
func (r *RealSystemProbe) Enable(initName string) error {
	return execx.Run(execx.Quick, "/etc/init.d/"+initName, "enable")
}
func (r *RealSystemProbe) Disable(initName string) error {
	return execx.Run(execx.Quick, "/etc/init.d/"+initName, "disable")
}

// --- Mock implementations for tests ---

// MockPackageManager tracks install/remove state in memory.
type MockPackageManager struct {
	installed   map[string]bool
	UpdateCalls int
	UpdateErr   error
}

func NewMockPackageManager() *MockPackageManager {
	return &MockPackageManager{installed: make(map[string]bool)}
}

func (m *MockPackageManager) Update() (string, error) {
	m.UpdateCalls++
	if m.UpdateErr != nil {
		return "", m.UpdateErr
	}
	return "ok", nil
}
func (m *MockPackageManager) Install(pkg string) (string, error) {
	m.installed[pkg] = true
	return "ok", nil
}
func (m *MockPackageManager) Remove(pkg string) (string, error) {
	delete(m.installed, pkg)
	return "ok", nil
}
func (m *MockPackageManager) IsInstalled(pkg string) bool {
	return m.installed[pkg]
}
func (m *MockPackageManager) InstallStream(pkg string, logFn func(string)) error {
	logFn("Installing " + pkg + "...")
	m.installed[pkg] = true
	logFn("Package " + pkg + " installed successfully")
	return nil
}
func (m *MockPackageManager) RemoveStream(pkg string, logFn func(string)) error {
	logFn("Removing " + pkg + "...")
	delete(m.installed, pkg)
	logFn("Package " + pkg + " removed successfully")
	return nil
}

// MockSystemProbe tracks init.d state in memory.
type MockSystemProbe struct {
	scripts   map[string]bool
	running   map[string]bool
	autoStart map[string]bool
}

func NewMockSystemProbe() *MockSystemProbe {
	return &MockSystemProbe{
		scripts:   make(map[string]bool),
		running:   make(map[string]bool),
		autoStart: make(map[string]bool),
	}
}
func (m *MockSystemProbe) HasInitScript(name string) bool { return m.scripts[name] }
func (m *MockSystemProbe) IsRunning(initName string) bool { return m.running[initName] }
func (m *MockSystemProbe) Start(initName string) (string, error) {
	m.running[initName] = true
	return "ok", nil
}
func (m *MockSystemProbe) Stop(initName string) (string, error) {
	delete(m.running, initName)
	return "ok", nil
}
func (m *MockSystemProbe) IsAutoStart(initName string) bool { return m.autoStart[initName] }
func (m *MockSystemProbe) Enable(initName string) error {
	m.autoStart[initName] = true
	return nil
}
func (m *MockSystemProbe) Disable(initName string) error {
	delete(m.autoStart, initName)
	return nil
}
