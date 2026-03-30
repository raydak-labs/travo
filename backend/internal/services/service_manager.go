package services

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// PackageManager abstracts package install/remove operations.
type PackageManager interface {
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
		ID:          "sqm",
		Name:        "SQM (Traffic Shaping)",
		Description: "Smart Queue Management to reduce latency (bufferbloat)",
		Packages:    []string{"sqm-scripts"},
		InitName:    "sqm",
	},
}

// ServiceManager manages installable services.
type ServiceManager struct {
	mu               sync.RWMutex
	defs             []serviceDefinition
	pkg              PackageManager
	probe            SystemProbe
	cache            map[string]models.ServiceInfo
	postInstallHooks map[string]func() error
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

// refreshOne updates the cache for a single service (must hold write lock).
func (sm *ServiceManager) refreshOne(serviceID string) {
	for _, def := range sm.defs {
		if def.ID == serviceID {
			sm.cache[def.ID] = sm.buildInfo(def)
			return
		}
	}
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

// Install installs the packages for a service.
func (sm *ServiceManager) Install(serviceID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	def, err := sm.findDef(serviceID)
	if err != nil {
		return err
	}
	for _, pkg := range def.Packages {
		if out, err := sm.pkg.Install(pkg); err != nil {
			return fmt.Errorf("failed to install %s: %w\n%s", pkg, err, out)
		}
	}
	sm.refreshOne(serviceID)
	if hook, ok := sm.postInstallHooks[serviceID]; ok {
		_ = hook() // Non-fatal: log but don't fail install
	}
	return nil
}

// Remove removes the packages for a service.
func (sm *ServiceManager) Remove(serviceID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	def, err := sm.findDef(serviceID)
	if err != nil {
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
	return nil
}

// InstallWithLog installs packages and streams output line by line via logFn.
func (sm *ServiceManager) InstallWithLog(serviceID string, logFn func(string)) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	def, err := sm.findDef(serviceID)
	if err != nil {
		return err
	}
	for _, pkg := range def.Packages {
		logFn(fmt.Sprintf("Installing package: %s", pkg))
		if err := sm.pkg.InstallStream(pkg, logFn); err != nil {
			return fmt.Errorf("failed to install %s: %w", pkg, err)
		}
	}
	sm.refreshOne(serviceID)
	if hook, ok := sm.postInstallHooks[serviceID]; ok {
		logFn("Running post-install configuration…")
		if err := hook(); err != nil {
			logFn(fmt.Sprintf("Post-install warning: %s", err.Error()))
		} else {
			logFn("Post-install configuration complete.")
		}
	}
	return nil
}

// RemoveWithLog removes packages and streams output line by line via logFn.
func (sm *ServiceManager) RemoveWithLog(serviceID string, logFn func(string)) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	def, err := sm.findDef(serviceID)
	if err != nil {
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
	return nil
}

// Start starts a service via init.d.
func (sm *ServiceManager) Start(serviceID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
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

// Stop stops a service via init.d.
func (sm *ServiceManager) Stop(serviceID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
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
	sm.mu.Lock()
	defer sm.mu.Unlock()
	def, err := sm.findDef(serviceID)
	if err != nil {
		return err
	}
	if def.InitName == "" {
		return fmt.Errorf("service %s does not have an init script", serviceID)
	}
	info, ok := sm.cache[def.ID]
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

func (sm *ServiceManager) findDef(serviceID string) (serviceDefinition, error) {
	for _, def := range sm.defs {
		if def.ID == serviceID {
			return def, nil
		}
	}
	return serviceDefinition{}, fmt.Errorf("service not found: %s", serviceID)
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

func (a *ApkPackageManager) Install(pkg string) (string, error) {
	out, err := exec.Command("apk", "add", pkg).CombinedOutput()
	return string(out), err
}
func (a *ApkPackageManager) Remove(pkg string) (string, error) {
	out, err := exec.Command("apk", "del", pkg).CombinedOutput()
	return string(out), err
}
func (a *ApkPackageManager) IsInstalled(pkg string) bool {
	err := exec.Command("apk", "info", "-e", pkg).Run()
	return err == nil
}
func (a *ApkPackageManager) InstallStream(pkg string, logFn func(string)) error {
	return streamCommand(exec.Command("apk", "add", pkg), logFn)
}
func (a *ApkPackageManager) RemoveStream(pkg string, logFn func(string)) error {
	return streamCommand(exec.Command("apk", "del", pkg), logFn)
}

// OpkgPackageManager uses opkg (OpenWrt <25).
type OpkgPackageManager struct{}

func (o *OpkgPackageManager) Install(pkg string) (string, error) {
	out, err := exec.Command("opkg", "install", pkg).CombinedOutput()
	return string(out), err
}
func (o *OpkgPackageManager) Remove(pkg string) (string, error) {
	out, err := exec.Command("opkg", "remove", pkg).CombinedOutput()
	return string(out), err
}
func (o *OpkgPackageManager) IsInstalled(pkg string) bool {
	out, err := exec.Command("opkg", "list-installed", pkg).CombinedOutput()
	return err == nil && strings.Contains(string(out), pkg)
}
func (o *OpkgPackageManager) InstallStream(pkg string, logFn func(string)) error {
	return streamCommand(exec.Command("opkg", "install", pkg), logFn)
}
func (o *OpkgPackageManager) RemoveStream(pkg string, logFn func(string)) error {
	return streamCommand(exec.Command("opkg", "remove", pkg), logFn)
}

// NoopPackageManager for systems without a package manager.
type NoopPackageManager struct{}

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

// streamCommand runs a command and sends each output line to logFn.
func streamCommand(cmd *exec.Cmd, logFn func(string)) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		logFn(scanner.Text())
	}

	return cmd.Wait()
}

// RealSystemProbe checks init.d scripts and running processes.
type RealSystemProbe struct{}

func (r *RealSystemProbe) HasInitScript(name string) bool {
	_, err := os.Stat("/etc/init.d/" + name)
	return err == nil
}
func (r *RealSystemProbe) IsRunning(initName string) bool {
	// OpenWrt init.d scripts return 0 for "running" status
	err := exec.Command("/etc/init.d/"+initName, "status").Run()
	return err == nil
}
func (r *RealSystemProbe) Start(initName string) (string, error) {
	out, err := exec.Command("/etc/init.d/"+initName, "start").CombinedOutput()
	return string(out), err
}
func (r *RealSystemProbe) Stop(initName string) (string, error) {
	out, err := exec.Command("/etc/init.d/"+initName, "stop").CombinedOutput()
	return string(out), err
}
func (r *RealSystemProbe) IsAutoStart(initName string) bool {
	err := exec.Command("/etc/init.d/"+initName, "enabled").Run()
	return err == nil
}
func (r *RealSystemProbe) Enable(initName string) error {
	return exec.Command("/etc/init.d/"+initName, "enable").Run()
}
func (r *RealSystemProbe) Disable(initName string) error {
	return exec.Command("/etc/init.d/"+initName, "disable").Run()
}

// --- Mock implementations for tests ---

// MockPackageManager tracks install/remove state in memory.
type MockPackageManager struct {
	installed map[string]bool
}

func NewMockPackageManager() *MockPackageManager {
	return &MockPackageManager{installed: make(map[string]bool)}
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
