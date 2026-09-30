package services

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/execx"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// usbCandidateInterfaces lists common names for USB tethering interfaces.
// usb0/usb1 = Android RNDIS; eth1/eth2 = NCM or iOS ipheth.
var usbCandidateInterfaces = []string{"usb0", "usb1", "eth1", "eth2"}

const usbTetherUCIName = "usbtether"

// usbTetherGuardPath is the crash guard for USB tether (re)configuration.
// It must exist while the live network/firewall state is being changed and is
// removed only after the change completed successfully (ADR 0003).
const usbTetherGuardPath = crashGuardDir + "/usbtether-in-progress"

// usbTetherConfigs is every UCI config this service mutates.
var usbTetherConfigs = []string{"firewall", "network"}

// USBTetherStatus holds the detected USB tethering state.
type USBTetherStatus struct {
	Detected   bool   `json:"detected"`
	DeviceType string `json:"device_type"` // "android", "ios", or "unknown"
	Interface  string `json:"interface"`
	IsUp       bool   `json:"is_up"`
	IPAddress  string `json:"ip_address"`
	Configured bool   `json:"configured"` // true when UCI usbtether interface exists
}

// USBTetherRunner abstracts OS calls for testability.
type USBTetherRunner interface {
	ReadSymlink(path string) (string, error)
	ReadFile(path string) (string, error)
	DirExists(path string) bool
	GetIfaceIP(iface string) string
	IsIfaceUp(iface string) bool
	RunCommand(name string, args ...string) (string, error)
}

// RealUSBTetherRunner uses the real OS.
type RealUSBTetherRunner struct{}

func (r *RealUSBTetherRunner) ReadSymlink(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

func (r *RealUSBTetherRunner) ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	return strings.TrimSpace(string(data)), err
}

func (r *RealUSBTetherRunner) DirExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (r *RealUSBTetherRunner) GetIfaceIP(iface string) string {
	ifaces, err := net.InterfaceByName(iface)
	if err != nil {
		return ""
	}
	addrs, err := ifaces.Addrs()
	if err != nil || len(addrs) == 0 {
		return ""
	}
	for _, addr := range addrs {
		if ip, ok := addr.(*net.IPNet); ok && ip.IP.To4() != nil {
			return ip.IP.String()
		}
	}
	return ""
}

func (r *RealUSBTetherRunner) IsIfaceUp(iface string) bool {
	ifc, err := net.InterfaceByName(iface)
	if err != nil {
		return false
	}
	return ifc.Flags&net.FlagUp != 0
}

func (r *RealUSBTetherRunner) RunCommand(name string, args ...string) (string, error) {
	out, err := execx.CombinedOutput(execx.Slow, name, args...)
	return strings.TrimSpace(string(out)), err
}

// USBTetheringService detects and configures USB-tethered devices.
type USBTetheringService struct {
	runner USBTetherRunner
	// guardFile is the crash-guard path (overridable in tests).
	guardFile string
}

// NewUSBTetheringService creates a service backed by the real system.
func NewUSBTetheringService() *USBTetheringService {
	return &USBTetheringService{runner: &RealUSBTetherRunner{}, guardFile: usbTetherGuardPath}
}

// NewUSBTetheringServiceWithRunner creates a service with a custom runner (tests).
func NewUSBTetheringServiceWithRunner(r USBTetherRunner) *USBTetheringService {
	return &USBTetheringService{runner: r, guardFile: usbTetherGuardPath}
}

// writeGuard creates the crash guard file before mutating live state.
func (s *USBTetheringService) writeGuard() error {
	if err := os.MkdirAll(filepath.Dir(s.guardFile), 0750); err != nil {
		return fmt.Errorf("create usb tether guard dir: %w", err)
	}
	if err := os.WriteFile(s.guardFile, []byte(time.Now().Format(time.RFC3339Nano)), 0600); err != nil {
		return fmt.Errorf("write usb tether guard: %w", err)
	}
	return nil
}

// clearGuard removes the crash guard after a successful change.
func (s *USBTetheringService) clearGuard() {
	if s.guardFile == "" {
		return
	}
	_ = os.Remove(s.guardFile)
}

// parseUciShow parses `uci show <prefix>` output into section -> option -> values.
// List options appear once per value, anonymous sections are keyed as "@zone[0]".
func parseUciShow(prefix, output string) map[string]map[string][]string {
	sections := map[string]map[string][]string{}
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix+".") {
			continue
		}
		name, value, found := strings.Cut(strings.TrimPrefix(line, prefix+"."), "=")
		if !found {
			continue
		}
		section, option, ok := strings.Cut(name, ".")
		if !ok {
			// Section header line, e.g. "firewall.@zone[0]=zone".
			continue
		}
		opts, exists := sections[section]
		if !exists {
			opts = map[string][]string{}
			sections[section] = opts
		}
		// uci prints a list as option='a' 'b'; trimming only the outer quotes
		// would yield the single element "a' 'b", so a multi-network WAN zone
		// never matches and Unconfigure skips its del_list.
		opts[option] = append(opts[option], uci.SplitUciValue(value)...)
	}
	return sections
}

// wanZoneSection resolves the firewall WAN zone by its `name` option instead of
// assuming a fixed index (the same resolution WifiService uses), and returns the
// zone's current network list.
func (s *USBTetheringService) wanZoneSection() (string, []string, error) {
	out, err := s.runner.RunCommand("uci", "show", "firewall")
	if err != nil {
		return "", nil, fmt.Errorf("uci show firewall: %w", err)
	}
	sections := parseUciShow("firewall", out)
	names := make([]string, 0, len(sections))
	for name := range sections {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		opts := sections[name]
		sectionType := ""
		if len(opts[".type"]) > 0 {
			sectionType = opts[".type"][0]
		}
		zoneName := ""
		if len(opts["name"]) > 0 {
			zoneName = opts["name"][0]
		}
		isZone := sectionType == "zone" || len(opts["input"]) > 0
		if !isZone || zoneName != "wan" {
			continue
		}
		return name, opts["network"], nil
	}
	return "", nil, errors.New("wan firewall zone not found")
}

// isUSBInterface returns true when the kernel interface is backed by a USB device.
func (s *USBTetheringService) isUSBInterface(name string) bool {
	devicePath := fmt.Sprintf("/sys/class/net/%s/device", name)
	if !s.runner.DirExists(devicePath) {
		return false
	}
	resolved, err := s.runner.ReadSymlink(devicePath)
	if err != nil {
		return false
	}
	return strings.Contains(resolved, "/usb")
}

// guessDeviceType returns a rough classification based on the interface name.
func guessDeviceType(name string) string {
	if strings.HasPrefix(name, "usb") {
		return "android"
	}
	// eth1+ could be iOS ipheth or Android NCM.
	return "android"
}

// GetStatus returns the current USB tethering detection state.
func (s *USBTetheringService) GetStatus() USBTetherStatus {
	for _, candidate := range usbCandidateInterfaces {
		if !s.isUSBInterface(candidate) {
			continue
		}
		// Found a USB-backed interface.
		configured := s.isConfigured()
		return USBTetherStatus{
			Detected:   true,
			DeviceType: guessDeviceType(candidate),
			Interface:  candidate,
			IsUp:       s.runner.IsIfaceUp(candidate),
			IPAddress:  s.runner.GetIfaceIP(candidate),
			Configured: configured,
		}
	}
	return USBTetherStatus{Detected: false}
}

// isConfigured returns true when UCI has a usbtether network interface.
func (s *USBTetheringService) isConfigured() bool {
	out, err := s.runner.RunCommand("uci", "get", fmt.Sprintf("network.%s", usbTetherUCIName))
	return err == nil && strings.TrimSpace(out) != ""
}

// Configure creates a UCI DHCP interface for the detected USB tethering device
// and adds it to the WAN firewall zone.
// Configure writes `network` (the usbtether interface) and `firewall` (adding it
// to the WAN zone) through shelled `uci`.
//
// Takes both locks. This is the only place found that mutates `network` and
// `firewall` outside the lock model, and it is invisible to the usual audit
// because it never goes through uci.Set — it shells out. Unlocked, its
// `uci commit firewall` can commit a VPN toggle's half-torn-down wg0 zone.
//
// withConfigLocks, not mutateUCI: no uci.UCI handle here to revert through, so a
// failure leaves this flow's staged delta for the next writer of those configs.
func (s *USBTetheringService) Configure(ifaceName string) error {
	return withConfigLocks(usbTetherConfigs, func() error { return s.configureLocked(ifaceName) })
}

func (s *USBTetheringService) configureLocked(ifaceName string) error {
	if strings.TrimSpace(ifaceName) == "" {
		return errors.New("usb tethering interface name is required")
	}
	if err := s.writeGuard(); err != nil {
		return err
	}
	cmds := [][]string{
		{"uci", "set", fmt.Sprintf("network.%s=interface", usbTetherUCIName)},
		{"uci", "set", fmt.Sprintf("network.%s.proto=dhcp", usbTetherUCIName)},
		{"uci", "set", fmt.Sprintf("network.%s.device=%s", usbTetherUCIName, ifaceName)},
		{"uci", "set", fmt.Sprintf("network.%s.metric=30", usbTetherUCIName)},
	}
	for _, args := range cmds {
		if _, err := s.runner.RunCommand(args[0], args[1:]...); err != nil {
			s.clearGuard()
			return fmt.Errorf("uci set failed (%v): %w", args, err)
		}
	}

	// Add usbtether to the WAN zone (add_list is idempotent).
	wanZone, zoneNetworks, err := s.wanZoneSection()
	if err != nil {
		s.clearGuard()
		return err
	}
	if !slices.Contains(zoneNetworks, usbTetherUCIName) {
		if _, err := s.runner.RunCommand("uci", "add_list", fmt.Sprintf("firewall.%s.network=%s", wanZone, usbTetherUCIName)); err != nil {
			s.clearGuard()
			return fmt.Errorf("uci add_list firewall wan zone: %w", err)
		}
	}

	if _, err := s.runner.RunCommand("uci", "commit", "network"); err != nil {
		s.clearGuard()
		return fmt.Errorf("uci commit network: %w", err)
	}
	if _, err := s.runner.RunCommand("uci", "commit", "firewall"); err != nil {
		s.clearGuard()
		return fmt.Errorf("uci commit firewall: %w", err)
	}

	// Bring up the interface. Best effort: the UCI config is committed and
	// netifd retries on its own, so a transient ifup failure is not fatal.
	_, _ = s.runner.RunCommand("ifup", usbTetherUCIName)

	s.clearGuard()
	return nil
}

// Unconfigure removes the usbtether UCI interface and its WAN zone reference.
// Unconfigure removes the interface and its firewall membership; see Configure
// for why it is locked.
func (s *USBTetheringService) Unconfigure() error {
	return withConfigLocks(usbTetherConfigs, func() error { return s.unconfigureLocked() })
}

func (s *USBTetheringService) unconfigureLocked() error {
	if err := s.writeGuard(); err != nil {
		return err
	}
	_, _ = s.runner.RunCommand("ifdown", usbTetherUCIName)

	// Remove the firewall reference first so a failure never leaves a dangling
	// network in the WAN zone after the interface is gone.
	wanZone, zoneNetworks, err := s.wanZoneSection()
	if err != nil {
		s.clearGuard()
		return err
	}
	if slices.Contains(zoneNetworks, usbTetherUCIName) {
		if _, err := s.runner.RunCommand("uci", "del_list", fmt.Sprintf("firewall.%s.network=%s", wanZone, usbTetherUCIName)); err != nil {
			s.clearGuard()
			return fmt.Errorf("uci del_list firewall wan zone: %w", err)
		}
		if _, err := s.runner.RunCommand("uci", "commit", "firewall"); err != nil {
			s.clearGuard()
			return fmt.Errorf("uci commit firewall: %w", err)
		}
	}

	if _, err := s.runner.RunCommand("uci", "delete", fmt.Sprintf("network.%s", usbTetherUCIName)); err != nil {
		s.clearGuard()
		return fmt.Errorf("uci delete: %w", err)
	}
	if _, err := s.runner.RunCommand("uci", "commit", "network"); err != nil {
		s.clearGuard()
		return fmt.Errorf("uci commit network: %w", err)
	}
	s.clearGuard()
	return nil
}
