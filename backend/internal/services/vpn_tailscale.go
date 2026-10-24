package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

const openwrtTailscaleBin = "/usr/sbin/tailscale"

const openwrtTailscaleBinAlt = "/usr/bin/tailscale"

// tailscaleInitScript is the procd service for the tailscale package.
const tailscaleInitScript = "/etc/init.d/tailscale"

// ErrTailscaleNotInstalled is returned when the tailscale package is absent, so
// none of its operations can run. Handlers map this to 503.
//
// Without it, every Tailscale mutation surfaced the raw exec error
// ("exec: \"tailscale\": executable file not found in $PATH") as a 500, which
// reads as a server fault and tells the operator nothing. Verified on the
// device: PUT /api/v1/vpn/tailscale/ssh returned
// 500 {"error":"exec: \"tailscale\": executable file not found in $PATH"}.
var ErrTailscaleNotInstalled = errors.New("tailscale is not installed on this router")

// TailscaleInstalled reports whether the tailscale package is present.
//
// Either artefact is enough: the binary is what the `tailscale` subcommands need,
// and the init script is what the start/stop toggle needs. A router can have one
// without the other mid-upgrade, so both are checked rather than assuming.
func TailscaleInstalled() bool {
	for _, p := range []string{openwrtTailscaleBin, openwrtTailscaleBinAlt, tailscaleInitScript} {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

// requireTailscale returns ErrTailscaleNotInstalled when the package is absent, so
// a Tailscale mutation fails with an actionable message instead of an exec error.
func requireTailscale() error {
	if !TailscaleInstalled() {
		return fmt.Errorf("%w: install the tailscale package to use Tailscale", ErrTailscaleNotInstalled)
	}
	return nil
}

func tailscaleBin() string {
	if _, err := os.Stat(openwrtTailscaleBin); err == nil {
		return openwrtTailscaleBin
	}
	if _, err := os.Stat(openwrtTailscaleBinAlt); err == nil {
		return openwrtTailscaleBinAlt
	}
	return "tailscale"
}

// tailscaleStatusJSON is the subset of `tailscale status --json` we parse.
type tailscaleStatusJSON struct {
	BackendState string   `json:"BackendState"`
	AuthURL      string   `json:"AuthURL"`
	TailscaleIPs []string `json:"TailscaleIPs"`
	Self         struct {
		DNSName      string   `json:"DNSName"`
		TailscaleIPs []string `json:"TailscaleIPs"`
		Online       bool     `json:"Online"`
	} `json:"Self"`
	Peer map[string]struct {
		DNSName        string   `json:"DNSName"`
		OS             string   `json:"OS"`
		Online         bool     `json:"Online"`
		ExitNode       bool     `json:"ExitNode"`
		ExitNodeOption bool     `json:"ExitNodeOption"`
		TailscaleIPs   []string `json:"TailscaleIPs"`
		LastSeen       string   `json:"LastSeen"`
	} `json:"Peer"`
}

// isTailscaleInstalled checks whether the tailscale binary exists.
func (v *VpnService) isTailscaleInstalled() bool {
	if _, err := os.Stat(openwrtTailscaleBin); err == nil {
		return true
	}
	if _, err := os.Stat(openwrtTailscaleBinAlt); err == nil {
		return true
	}
	// Fall back to PATH check for non-OpenWrt/dev environments.
	_, err := v.cmd.Run("which", "tailscale")
	return err == nil
}

// isTailscaleRunning checks whether tailscaled is running.
func (v *VpnService) isTailscaleRunning() bool {
	_, err := v.cmd.Run("/etc/init.d/tailscale", "status")
	return err == nil
}

// GetTailscaleStatus returns Tailscale status, including peers when logged in.
func (v *VpnService) GetTailscaleStatus() (models.TailscaleStatus, error) {
	installed := v.isTailscaleInstalled()
	if !installed {
		return models.TailscaleStatus{
			Installed: false,
			Running:   false,
			LoggedIn:  false,
			Peers:     []models.TailscalePeer{},
		}, nil
	}

	running := v.isTailscaleRunning()
	if !running {
		return models.TailscaleStatus{
			Installed: true,
			Running:   false,
			LoggedIn:  false,
			Peers:     []models.TailscalePeer{},
		}, nil
	}

	raw, err := v.cmd.Run(tailscaleBin(), "status", "--json")
	if err != nil {
		return models.TailscaleStatus{Installed: true, Running: true, Peers: []models.TailscalePeer{}}, nil
	}

	var ts tailscaleStatusJSON
	if err := json.Unmarshal(raw, &ts); err != nil {
		return models.TailscaleStatus{Installed: true, Running: true, Peers: []models.TailscalePeer{}}, nil
	}

	loggedIn := ts.BackendState == "Running" || ts.BackendState == "Starting"

	var ip string
	if len(ts.Self.TailscaleIPs) > 0 {
		ip = ts.Self.TailscaleIPs[0]
	} else if len(ts.TailscaleIPs) > 0 {
		ip = ts.TailscaleIPs[0]
	}

	hostname := strings.TrimSuffix(ts.Self.DNSName, ".")
	if idx := strings.Index(hostname, "."); idx >= 0 {
		hostname = hostname[:idx]
	}

	// Build peers list.
	peers := make([]models.TailscalePeer, 0, len(ts.Peer))
	for _, p := range ts.Peer {
		var peerIP string
		if len(p.TailscaleIPs) > 0 {
			peerIP = p.TailscaleIPs[0]
		}
		peerHostname := strings.TrimSuffix(p.DNSName, ".")
		if idx := strings.Index(peerHostname, "."); idx >= 0 {
			peerHostname = peerHostname[:idx]
		}
		peers = append(peers, models.TailscalePeer{
			Hostname:       peerHostname,
			TailscaleIP:    peerIP,
			OS:             p.OS,
			Online:         p.Online,
			ExitNode:       p.ExitNode,
			ExitNodeOption: p.ExitNodeOption,
			LastSeen:       p.LastSeen,
		})
	}

	authURL := ""
	if ts.BackendState == "NeedsLogin" || ts.AuthURL != "" {
		authURL = ts.AuthURL
	}

	return models.TailscaleStatus{
		Installed: true,
		Running:   true,
		LoggedIn:  loggedIn,
		IPAddress: ip,
		Hostname:  hostname,
		Peers:     peers,
		AuthURL:   authURL,
	}, nil
}

// StartTailscaleAuth runs `tailscale up` and returns the auth URL if login is required.
func (v *VpnService) StartTailscaleAuth(authKey string) (string, error) {
	if err := requireTailscale(); err != nil {
		return "", err
	}
	args := []string{"up", "--accept-routes"}
	if authKey != "" {
		args = append(args, "--auth-key="+authKey)
	}
	out, _ := v.cmd.Run(tailscaleBin(), args...)
	combined := strings.TrimSpace(string(out))
	// Extract URL from output like "To authenticate, visit:\n\thttps://login.tailscale.com/..."
	for line := range strings.SplitSeq(combined, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "https://login.tailscale.com") || strings.HasPrefix(line, "https://tailscale.com") {
			return line, nil
		}
	}
	// Also check the status for an auth URL.
	status, _ := v.GetTailscaleStatus()
	return status.AuthURL, nil
}

// SetTailscaleExitNode sets or clears the Tailscale exit node.
// When a non-empty exit node is set, WireGuard is turned off first so only one
// full-tunnel-style path is active (see requirements: single active VPN policy).
func (v *VpnService) SetTailscaleExitNode(nodeIP string) error {
	if err := requireTailscale(); err != nil {
		return err
	}
	nodeIP = strings.TrimSpace(nodeIP)
	if nodeIP != "" {
		if opts, err := v.uci.GetAll("network", "wg0"); err == nil && opts["disabled"] != "1" {
			if err := v.ToggleWireguard(false); err != nil {
				return fmt.Errorf("disable WireGuard before using a Tailscale exit node: %w", err)
			}
		}
	}
	if nodeIP == "" {
		_, err := v.cmd.Run(tailscaleBin(), "set", "--exit-node=")
		return err
	}
	_, err := v.cmd.Run(tailscaleBin(), "set", "--exit-node="+nodeIP, "--exit-node-allow-lan-access=true")
	return err
}

// ToggleTailscale starts or stops the Tailscale daemon via init.d.
func (v *VpnService) ToggleTailscale(enable bool) error {
	if err := requireTailscale(); err != nil {
		return err
	}
	if enable {
		_, err := v.cmd.Run("/etc/init.d/tailscale", "start")
		return err
	}
	_, err := v.cmd.Run("/etc/init.d/tailscale", "stop")
	return err
}

// GetTailscaleSSHEnabled returns whether Tailscale SSH is enabled.
func (v *VpnService) GetTailscaleSSHEnabled() (bool, error) {
	out, err := v.cmd.Run(tailscaleBin(), "status", "--json")
	if err != nil {
		return false, nil // Tailscale not running
	}
	var status struct {
		Prefs struct {
			RunSSH bool `json:"RunSSH"`
		} `json:"Prefs"`
	}
	if err := json.Unmarshal(out, &status); err != nil {
		return false, nil
	}
	return status.Prefs.RunSSH, nil
}

// SetTailscaleSSHEnabled enables or disables Tailscale SSH.
func (v *VpnService) SetTailscaleSSHEnabled(enabled bool) error {
	if err := requireTailscale(); err != nil {
		return err
	}
	arg := "--ssh=false"
	if enabled {
		arg = "--ssh"
	}
	_, err := v.cmd.Run(tailscaleBin(), "set", arg)
	return err
}
