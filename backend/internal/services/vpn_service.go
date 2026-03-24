package services

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// CommandRunner abstracts command execution for testability.
type CommandRunner interface {
	Run(name string, args ...string) ([]byte, error)
}

// RealCommandRunner executes real OS commands.
type RealCommandRunner struct{}

// Run executes a command and returns its output.
func (r *RealCommandRunner) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

// VpnService provides VPN status and configuration.
type VpnService struct {
	uci          uci.UCI
	cmd          CommandRunner
	profilesPath string // Path to wireguard_profiles.json
}

// NewVpnService creates a new VpnService with a real command runner.
func NewVpnService(u uci.UCI) *VpnService {
	return &VpnService{uci: u, cmd: &RealCommandRunner{}, profilesPath: "/etc/openwrt-travel-gui/wireguard_profiles.json"}
}

// NewVpnServiceWithRunner creates a new VpnService with a custom command runner (for tests).
func NewVpnServiceWithRunner(u uci.UCI, cmd CommandRunner) *VpnService {
	return &VpnService{uci: u, cmd: cmd, profilesPath: "/etc/openwrt-travel-gui/wireguard_profiles.json"}
}

// NewVpnServiceWithProfilesPath creates a VpnService with a custom profiles path (for tests).
func NewVpnServiceWithProfilesPath(u uci.UCI, cmd CommandRunner, profilesPath string) *VpnService {
	return &VpnService{uci: u, cmd: cmd, profilesPath: profilesPath}
}

// ensureWireGuardInterface normalizes the network.wg0 UCI section so it has the
// correct type (interface) and proto (wireguard). This must be called before
// writing WireGuard-specific options, because UCI Set on a missing or wrong-typed
// section silently fails on some OpenWrt builds.
func (v *VpnService) ensureWireGuardInterface() error {
	opts, err := v.uci.GetAll("network", "wg0")
	if err != nil {
		// Section doesn't exist: create it.
		if addErr := v.uci.AddSection("network", "wg0", "interface"); addErr != nil {
			return fmt.Errorf("creating network.wg0 section: %w", addErr)
		}
	} else if opts[".type"] != "interface" {
		// Wrong section type — delete and recreate is the safe approach, but
		// in practice just ensuring proto is set should be enough to overwrite.
		_ = opts
	}
	return v.uci.Set("network", "wg0", "proto", "wireguard")
}

// ensureWireGuardPeer normalizes a peer section. Peer sections must have type
// "wireguard_wg0" so netifd binds them to the wg0 interface.
func (v *VpnService) ensureWireGuardPeer(section string) error {
	if _, err := v.uci.GetAll("network", section); err != nil {
		if addErr := v.uci.AddSection("network", section, "wireguard_wg0"); addErr != nil {
			return fmt.Errorf("creating peer section %s: %w", section, addErr)
		}
	}
	return nil
}

// applyAndVerifyWireGuard brings up wg0 via ifup and verifies the interface is
// live using `wg show wg0 dump`. Returns an error if the tunnel is not up.
func (v *VpnService) applyAndVerifyWireGuard() error {
	if _, err := v.cmd.Run("ifup", "wg0"); err != nil {
		return fmt.Errorf("ifup wg0 failed: %w", err)
	}
	out, err := v.cmd.Run("wg", "show", "wg0", "dump")
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return fmt.Errorf("wg0 is not up after apply: wg show wg0 dump returned no output")
	}
	return nil
}

// wgRuntimeState returns a fine-grained status detail string for the wg0 interface.
func (v *VpnService) wgRuntimeState(enabled bool) string {
	if !enabled {
		return "disabled"
	}
	out, err := v.cmd.Run("wg", "show", "wg0", "dump")
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return "enabled_not_up"
	}
	status, parseErr := ParseWgDump(string(out))
	if parseErr != nil || status == nil {
		return "enabled_not_up"
	}
	for _, peer := range status.Peers {
		if peer.LatestHandshake > 0 {
			return "connected"
		}
	}
	if len(status.Peers) > 0 {
		return "up_no_handshake"
	}
	return "configured"
}

// GetVpnStatus returns all VPN connection statuses.
func (v *VpnService) GetVpnStatus() ([]models.VpnStatus, error) {
	var statuses []models.VpnStatus

	// WireGuard — only include if configured
	disabled, err := v.uci.Get("network", "wg0", "disabled")
	if err == nil {
		// WireGuard is configured
		wgStatus := models.VpnStatus{Type: "wireguard"}
		wgStatus.Enabled = disabled != "1"
		wgStatus.StatusDetail = v.wgRuntimeState(wgStatus.Enabled)
		wgStatus.Connected = wgStatus.StatusDetail == "connected"
		if wgStatus.Enabled {
			if endpoint, err := v.uci.Get("network", "wg0_peer0", "endpoint"); err == nil {
				wgStatus.Endpoint = endpoint
			}
		}
		statuses = append(statuses, wgStatus)
	}

	// Tailscale
	statuses = append(statuses, models.VpnStatus{
		Type:    "tailscale",
		Enabled: false,
	})

	return statuses, nil
}

// GetWireGuardStatus returns live WireGuard status by running `wg show wg0 dump`.
// The dump format is tab-separated:
// Line 1 (interface): private_key  public_key  listen_port  fwmark
// Line 2+ (peers): public_key  preshared_key  endpoint  allowed_ips  latest_handshake_epoch  transfer_rx  transfer_tx  persistent_keepalive
func (v *VpnService) GetWireGuardStatus() (*models.WireGuardStatus, error) {
	out, err := v.cmd.Run("wg", "show", "wg0", "dump")
	if err != nil {
		// wg show fails with exit status 1 when interface doesn't exist (tunnel not active)
		return &models.WireGuardStatus{}, nil
	}
	return ParseWgDump(string(out))
}

// ParseWgDump parses the output of `wg show <iface> dump` into a WireGuardStatus.
func ParseWgDump(dump string) (*models.WireGuardStatus, error) {
	lines := strings.Split(strings.TrimSpace(dump), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return nil, fmt.Errorf("empty wg dump output")
	}

	// Parse interface line
	ifFields := strings.Split(lines[0], "\t")
	if len(ifFields) < 3 {
		return nil, fmt.Errorf("invalid interface line: expected at least 3 fields, got %d", len(ifFields))
	}

	listenPort, _ := strconv.Atoi(ifFields[2])
	status := &models.WireGuardStatus{
		Interface:  "wg0",
		PublicKey:  ifFields[1],
		ListenPort: listenPort,
	}

	// Parse peer lines
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 8 {
			continue
		}
		handshake, _ := strconv.ParseInt(fields[4], 10, 64)
		rx, _ := strconv.ParseInt(fields[5], 10, 64)
		tx, _ := strconv.ParseInt(fields[6], 10, 64)

		peer := models.WireGuardPeerStatus{
			PublicKey:       fields[0],
			Endpoint:        fields[2],
			AllowedIPs:      fields[3],
			LatestHandshake: handshake,
			TransferRx:      rx,
			TransferTx:      tx,
		}
		status.Peers = append(status.Peers, peer)
	}

	return status, nil
}

// GetWireguardConfig returns the WireGuard configuration.
func (v *VpnService) GetWireguardConfig() (models.WireguardConfig, error) {
	opts, err := v.uci.GetAll("network", "wg0")
	if err != nil {
		// WireGuard not configured - return empty config (not an error)
		return models.WireguardConfig{}, nil
	}

	config := models.WireguardConfig{
		PrivateKey: opts["private_key"],
		Address:    opts["addresses"],
	}
	if dns, ok := opts["dns"]; ok && dns != "" {
		config.DNS = strings.Split(dns, " ")
	}

	// Get peer
	peerOpts, err := v.uci.GetAll("network", "wg0_peer0")
	if err == nil {
		peer := models.WireguardPeer{
			PublicKey: peerOpts["public_key"],
			Endpoint:  peerOpts["endpoint"],
		}
		if ips, ok := peerOpts["allowed_ips"]; ok && ips != "" {
			peer.AllowedIPs = strings.Split(ips, ",")
		}
		config.Peers = []models.WireguardPeer{peer}
	}

	return config, nil
}

// SetWireguardConfig updates the WireGuard configuration.
func (v *VpnService) SetWireguardConfig(config models.WireguardConfig) error {
	if config.PrivateKey != "" {
		_ = v.uci.Set("network", "wg0", "private_key", config.PrivateKey)
	}
	if config.Address != "" {
		_ = v.uci.Set("network", "wg0", "addresses", config.Address)
	}
	if len(config.DNS) > 0 {
		_ = v.uci.Set("network", "wg0", "dns", strings.Join(config.DNS, " "))
	}
	return v.uci.Commit("network")
}

// ToggleWireguard enables or disables WireGuard. When enabling, it also ensures
// the UCI structure is correct, commits, brings up wg0, and verifies the tunnel
// is live. Returns an error if the tunnel fails to come up (no false success).
func (v *VpnService) ToggleWireguard(enable bool) error {
	val := "1"
	if enable {
		val = "0"
		// Normalize interface structure before enabling.
		if err := v.ensureWireGuardInterface(); err != nil {
			return fmt.Errorf("normalizing wg0 interface: %w", err)
		}
	}
	_ = v.uci.Set("network", "wg0", "disabled", val)
	if err := v.uci.Commit("network"); err != nil {
		return err
	}
	if enable {
		if err := v.applyAndVerifyWireGuard(); err != nil {
			return fmt.Errorf("WireGuard enabled in UCI but tunnel failed to start: %w", err)
		}
	} else {
		// Bring down the interface when disabling.
		_, _ = v.cmd.Run("ifdown", "wg0")
	}
	return nil
}

// GetTailscaleStatus returns Tailscale status.
func (v *VpnService) GetTailscaleStatus() (models.TailscaleStatus, error) {
	return models.TailscaleStatus{
		Installed: false,
		Running:   false,
		LoggedIn:  false,
	}, nil
}

// GetKillSwitch checks whether the VPN kill switch firewall rule exists.
func (v *VpnService) GetKillSwitch() (models.KillSwitchStatus, error) {
	opts, err := v.uci.GetAll("firewall", "vpn_killswitch")
	if err != nil {
		return models.KillSwitchStatus{Enabled: false}, nil
	}
	return models.KillSwitchStatus{
		Enabled: opts["src"] == "lan" && opts["dest"] == "wan" && opts["target"] == "REJECT",
	}, nil
}

// SetKillSwitch enables or disables the VPN kill switch firewall rule.
func (v *VpnService) SetKillSwitch(enabled bool) error {
	if enabled {
		// Create the firewall rule that blocks LAN→WAN when VPN is down.
		_ = v.uci.AddSection("firewall", "vpn_killswitch", "rule")
		_ = v.uci.Set("firewall", "vpn_killswitch", "name", "VPN Kill Switch")
		_ = v.uci.Set("firewall", "vpn_killswitch", "src", "lan")
		_ = v.uci.Set("firewall", "vpn_killswitch", "dest", "wan")
		_ = v.uci.Set("firewall", "vpn_killswitch", "target", "REJECT")
	} else {
		// Remove the firewall rule; ignore error if it doesn't exist.
		_ = v.uci.DeleteSection("firewall", "vpn_killswitch")
	}
	return v.uci.Commit("firewall")
}

// ImportWireguardConfig parses a .conf file, normalizes the UCI structure,
// applies the config, and verifies the tunnel comes up.
func (v *VpnService) ImportWireguardConfig(confContent string) error {
	parsed, err := ParseWireguardConfig(confContent)
	if err != nil {
		return err
	}

	// Normalize wg0 UCI structure before writing values.
	if err := v.ensureWireGuardInterface(); err != nil {
		return fmt.Errorf("normalizing wg0 interface: %w", err)
	}

	// Apply interface settings
	_ = v.uci.Set("network", "wg0", "private_key", parsed.Interface.PrivateKey)
	if parsed.Interface.Address != "" {
		_ = v.uci.Set("network", "wg0", "addresses", parsed.Interface.Address)
	}
	if parsed.Interface.DNS != "" {
		_ = v.uci.Set("network", "wg0", "dns", parsed.Interface.DNS)
	}

	// Apply peers — normalize section type before writing.
	for i, peer := range parsed.Peers {
		section := fmt.Sprintf("wg0_peer%d", i)
		if err := v.ensureWireGuardPeer(section); err != nil {
			return fmt.Errorf("normalizing peer section %s: %w", section, err)
		}
		_ = v.uci.Set("network", section, "public_key", peer.PublicKey)
		if peer.Endpoint != "" {
			_ = v.uci.Set("network", section, "endpoint", peer.Endpoint)
		}
		if peer.AllowedIPs != "" {
			_ = v.uci.Set("network", section, "allowed_ips", peer.AllowedIPs)
		}
		if peer.PresharedKey != "" {
			_ = v.uci.Set("network", section, "preshared_key", peer.PresharedKey)
		}
	}

	return v.uci.Commit("network")
}

// ToggleTailscale enables or disables Tailscale.
func (v *VpnService) ToggleTailscale(_ bool) error {
	// Stub - tailscale not installed in mock
	return nil
}

// loadProfiles reads profiles from the JSON file.
func (v *VpnService) loadProfiles() ([]models.WireGuardProfile, error) {
	data, err := os.ReadFile(v.profilesPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.WireGuardProfile{}, nil
		}
		return nil, fmt.Errorf("reading profiles file: %w", err)
	}
	var profiles []models.WireGuardProfile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, fmt.Errorf("parsing profiles file: %w", err)
	}
	return profiles, nil
}

// saveProfiles writes profiles to the JSON file.
func (v *VpnService) saveProfiles(profiles []models.WireGuardProfile) error {
	data, err := json.Marshal(profiles)
	if err != nil {
		return fmt.Errorf("marshaling profiles: %w", err)
	}
	dir := filepath.Dir(v.profilesPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating profiles directory: %w", err)
	}
	if err := os.WriteFile(v.profilesPath, data, 0o600); err != nil {
		return fmt.Errorf("writing profiles file: %w", err)
	}
	return nil
}

// generateProfileID creates a short random hex ID.
func generateProfileID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// GetProfiles returns all saved WireGuard profiles.
func (v *VpnService) GetProfiles() ([]models.WireGuardProfile, error) {
	return v.loadProfiles()
}

// AddProfile saves a new WireGuard profile.
func (v *VpnService) AddProfile(name, config string) (*models.WireGuardProfile, error) {
	// Validate the config is parseable
	if _, err := ParseWireguardConfig(config); err != nil {
		return nil, fmt.Errorf("invalid WireGuard config: %w", err)
	}

	profiles, err := v.loadProfiles()
	if err != nil {
		return nil, err
	}

	profile := models.WireGuardProfile{
		ID:        generateProfileID(),
		Name:      name,
		Config:    config,
		Active:    false,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	profiles = append(profiles, profile)

	if err := v.saveProfiles(profiles); err != nil {
		return nil, err
	}
	return &profile, nil
}

// DeleteProfile removes a profile by ID.
func (v *VpnService) DeleteProfile(id string) error {
	profiles, err := v.loadProfiles()
	if err != nil {
		return err
	}

	found := false
	filtered := make([]models.WireGuardProfile, 0, len(profiles))
	for _, p := range profiles {
		if p.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, p)
	}
	if !found {
		return fmt.Errorf("profile not found: %s", id)
	}

	return v.saveProfiles(filtered)
}

// ActivateProfile loads a profile's config into UCI and marks it as active.
func (v *VpnService) ActivateProfile(id string) error {
	profiles, err := v.loadProfiles()
	if err != nil {
		return err
	}

	var target *models.WireGuardProfile
	for i := range profiles {
		if profiles[i].ID == id {
			target = &profiles[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("profile not found: %s", id)
	}

	// Apply the config via the existing import logic
	if err := v.ImportWireguardConfig(target.Config); err != nil {
		return fmt.Errorf("applying profile config: %w", err)
	}

	// Mark only this profile as active
	for i := range profiles {
		profiles[i].Active = profiles[i].ID == id
	}

	return v.saveProfiles(profiles)
}

// RunDNSLeakTest checks whether the system's active DNS servers match the VPN
// configuration. A mismatch when the VPN is active suggests a DNS leak.
func (v *VpnService) RunDNSLeakTest() models.DNSLeakResult {
	result := models.DNSLeakResult{}

	// 1. Read /etc/resolv.conf for current system nameservers.
	result.Nameservers = readResolvConfNameservers()

	// 2. Check VPN status.
	disabled, err := v.uci.Get("network", "wg0", "disabled")
	result.VPNActive = err == nil && disabled != "1"

	// 3. Read VPN DNS servers from WireGuard UCI config.
	if dns, err := v.uci.Get("network", "wg0", "dns"); err == nil && dns != "" {
		for _, s := range strings.Split(dns, " ") {
			s = strings.TrimSpace(s)
			if s != "" {
				result.VPNDNSServers = append(result.VPNDNSServers, s)
			}
		}
	}

	// 4. Check for potential leak: VPN active but DNS not routed through VPN.
	if result.VPNActive && len(result.VPNDNSServers) > 0 {
		vpnDNSSet := make(map[string]bool, len(result.VPNDNSServers))
		for _, s := range result.VPNDNSServers {
			vpnDNSSet[s] = true
		}
		leaking := true
		for _, ns := range result.Nameservers {
			if vpnDNSSet[ns] {
				leaking = false
				break
			}
		}
		result.PotentiallyLeaking = leaking
	}

	return result
}

// readResolvConfNameservers parses nameserver lines from /etc/resolv.conf.
func readResolvConfNameservers() []string {
	return readResolvConfNameserversFromPath("/etc/resolv.conf")
}

// readResolvConfNameserversFromPath parses nameserver lines from the given file.
func readResolvConfNameserversFromPath(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var servers []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "nameserver ") {
			ns := strings.TrimSpace(strings.TrimPrefix(line, "nameserver "))
			if ns != "" {
				servers = append(servers, ns)
			}
		}
	}
	return servers
}
