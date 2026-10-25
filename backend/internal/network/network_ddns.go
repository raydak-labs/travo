package network

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
)

// GetDDNSConfig reads the DDNS configuration from UCI (ddns.myddns).
func (n *NetworkService) GetDDNSConfig() (models.DDNSConfig, error) {
	opts, err := n.uci.GetAll("ddns", "myddns")
	if err != nil {
		// No ddns config — return defaults (disabled)
		return models.DDNSConfig{}, nil
	}
	svc := opts["service_name"]
	updateURL := opts["update_url"]
	// LuCI/custom setups often use update_url with empty or "-" service_name.
	if updateURL != "" && (svc == "" || svc == "-") {
		svc = "custom"
	}
	return models.DDNSConfig{
		Enabled:    opts["enabled"] == "1",
		Service:    svc,
		Domain:     opts["domain"],
		Username:   opts["username"],
		Password:   opts["password"],
		LookupHost: opts["lookup_host"],
		UpdateURL:  updateURL,
	}, nil
}

// ddnsSectionName is the ddns-scripts service section this service owns.
const ddnsSectionName = "myddns"

// ErrDDNSNotAvailable is returned when ddns-scripts is not installed, so there
// is nothing that can act on a `ddns` UCI config. Handlers map this to 503 with
// an actionable message rather than surfacing uci's "Entry not found".
var ErrDDNSNotAvailable = errors.New("ddns-scripts is not installed on this router")

// ddnsInitScriptPath is the production ddns-scripts init script.
const ddnsInitScriptPath = initdPath + "ddns"

// SetDDNSInitScript overrides the ddns-scripts init script path (tests only).
// Tests that exercise DDNS configuration need it to point at a file that exists,
// otherwise every write correctly reports the package as missing.
func (n *NetworkService) SetDDNSInitScript(path string) {
	n.ddnsInitScript = path
}

// DDNSAvailable reports whether ddns-scripts is installed.
//
// Checked via the init script rather than the `ddns` UCI config, because the
// config is what the user is trying to create: on a device without the package
// there is no config AND no way for the UI to install one (ddns is not in the
// service catalog), so the only honest signal is the missing script.
func (n *NetworkService) DDNSAvailable() bool {
	path := n.ddnsInitScript
	if path == "" {
		path = ddnsInitScriptPath
	}
	_, err := os.Stat(path)
	return err == nil
}

// SetDDNSConfig writes the DDNS configuration to UCI and restarts ddns-scripts.
func (n *NetworkService) SetDDNSConfig(config models.DDNSConfig) error {
	// Check the precondition BEFORE touching UCI. The old order wrote half the
	// config and then failed, and on a device without the package every write
	// failed with a bare `uci: Entry not found` 500 that said nothing about the
	// real cause.
	if !n.DDNSAvailable() {
		return fmt.Errorf("%w: install the ddns-scripts package to configure dynamic DNS", ErrDDNSNotAvailable)
	}

	enabled := "0"
	if config.Enabled {
		enabled = "1"
	}

	// `ddns` is locked like every other UCI mutator: the uci CLI keeps
	// uncommitted changes in the process-global /tmp/.uci/ddns/changes file.
	return platform.MutateUCI(n.uci, []string{"ddns"}, func() error {
		// The `myddns` section is created on demand. It does not exist on a
		// router where DDNS was never configured, and `uci set` on a missing
		// section fails outright, so without this the endpoint could only ever
		// succeed on a device that already had DDNS set up.
		if _, err := n.uci.GetAll("ddns", ddnsSectionName); err != nil {
			if err := n.uci.AddSection("ddns", ddnsSectionName, "service"); err != nil {
				return fmt.Errorf("creating ddns section: %w", err)
			}
		}
		if err := n.uciSet("ddns", ddnsSectionName, "enabled", enabled); err != nil {
			return err
		}
		if strings.EqualFold(strings.TrimSpace(config.Service), "custom") {
			_ = n.uci.DeleteOption("ddns", ddnsSectionName, "service_name")
			if err := n.uciSet("ddns", ddnsSectionName, "update_url", strings.TrimSpace(config.UpdateURL)); err != nil {
				return err
			}
		} else {
			_ = n.uci.DeleteOption("ddns", ddnsSectionName, "update_url")
			if err := n.uciSet("ddns", ddnsSectionName, "service_name", config.Service); err != nil {
				return err
			}
		}
		if err := n.uciSet("ddns", ddnsSectionName, "domain", config.Domain); err != nil {
			return err
		}
		if err := n.uciSet("ddns", ddnsSectionName, "username", config.Username); err != nil {
			return err
		}
		if err := n.uciSet("ddns", ddnsSectionName, "password", config.Password); err != nil {
			return err
		}
		if err := n.uciSet("ddns", ddnsSectionName, "lookup_host", config.LookupHost); err != nil {
			return err
		}
		if err := n.uciCommit("ddns"); err != nil {
			return err
		}
		if err := n.restartService("ddns"); err != nil {
			return fmt.Errorf("restart ddns: %w", err)
		}
		return nil
	})
}

// GetDDNSStatus checks whether the ddns service is running and returns public IP info.
func (n *NetworkService) GetDDNSStatus() (models.DDNSStatus, error) {
	var status models.DDNSStatus

	// Check if ddns process is running
	if _, err := n.cmd.Run("pgrep", "-f", "ddns"); err == nil {
		status.Running = true
	}

	// Read cached public IP from ddns update file
	data, err := n.cmd.Run("cat", "/var/run/ddns/myddns.ip")
	if err == nil {
		ip := strings.TrimSpace(string(data))
		if ip != "" {
			status.PublicIP = ip
		}
	}

	// Read last update timestamp
	data, err = n.cmd.Run("cat", "/var/run/ddns/myddns.update")
	if err == nil {
		ts := strings.TrimSpace(string(data))
		if ts != "" {
			status.LastUpdate = ts
		}
	}

	return status, nil
}
