package services

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// usbCandidateInterfaces lists common names for USB tethering interfaces.
// usb0/usb1 = Android RNDIS; eth1/eth2 = NCM or iOS ipheth.
var usbCandidateInterfaces = []string{"usb0", "usb1", "eth1", "eth2"}

const usbTetherUCIName = "usbtether"

// USBTetherStatus holds the detected USB tethering state.
type USBTetherStatus struct {
	Detected    bool   `json:"detected"`
	DeviceType  string `json:"device_type"` // "android", "ios", or "unknown"
	Interface   string `json:"interface"`
	IsUp        bool   `json:"is_up"`
	IPAddress   string `json:"ip_address"`
	Configured  bool   `json:"configured"` // true when UCI usbtether interface exists
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
	out, err := exec.Command(name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// USBTetheringService detects and configures USB-tethered devices.
type USBTetheringService struct {
	runner USBTetherRunner
}

// NewUSBTetheringService creates a service backed by the real system.
func NewUSBTetheringService() *USBTetheringService {
	return &USBTetheringService{runner: &RealUSBTetherRunner{}}
}

// NewUSBTetheringServiceWithRunner creates a service with a custom runner (tests).
func NewUSBTetheringServiceWithRunner(r USBTetherRunner) *USBTetheringService {
	return &USBTetheringService{runner: r}
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
func (s *USBTetheringService) Configure(ifaceName string) error {
	cmds := [][]string{
		{"uci", "set", fmt.Sprintf("network.%s=interface", usbTetherUCIName)},
		{"uci", "set", fmt.Sprintf("network.%s.proto=dhcp", usbTetherUCIName)},
		{"uci", "set", fmt.Sprintf("network.%s.device=%s", usbTetherUCIName, ifaceName)},
		{"uci", "set", fmt.Sprintf("network.%s.metric=30", usbTetherUCIName)},
	}
	for _, args := range cmds {
		if _, err := s.runner.RunCommand(args[0], args[1:]...); err != nil {
			return fmt.Errorf("uci set failed (%v): %w", args, err)
		}
	}

	// Add usbtether to WAN zone (add_list is idempotent).
	_, _ = s.runner.RunCommand("uci", "add_list", fmt.Sprintf("firewall.@zone[1].network=%s", usbTetherUCIName))

	if _, err := s.runner.RunCommand("uci", "commit", "network"); err != nil {
		return fmt.Errorf("uci commit network: %w", err)
	}
	_, _ = s.runner.RunCommand("uci", "commit", "firewall")

	// Bring up the interface.
	_, _ = s.runner.RunCommand("ifup", usbTetherUCIName)

	return nil
}

// Unconfigure removes the usbtether UCI interface.
func (s *USBTetheringService) Unconfigure() error {
	_, _ = s.runner.RunCommand("ifdown", usbTetherUCIName)
	if _, err := s.runner.RunCommand("uci", "delete", fmt.Sprintf("network.%s", usbTetherUCIName)); err != nil {
		return fmt.Errorf("uci delete: %w", err)
	}
	if _, err := s.runner.RunCommand("uci", "commit", "network"); err != nil {
		return fmt.Errorf("uci commit network: %w", err)
	}
	return nil
}
