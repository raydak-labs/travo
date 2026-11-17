package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/uplink"
)

type repairAdGuardChecker struct {
	files   map[string]bool
	content map[string]string
	calls   []string
}

func (m *repairAdGuardChecker) FileExists(path string) bool { return m.files[path] }

func (m *repairAdGuardChecker) RunCommand(name string, args ...string) (string, error) {
	call := name
	if len(args) > 0 {
		call += " " + strings.Join(args, " ")
	}
	m.calls = append(m.calls, call)
	switch call {
	case "uci -q get adguardhome.config.config_file":
		return "/etc/adguardhome/adguardhome.yaml", nil
	case "uci get dhcp.@dnsmasq[0].server":
		return "127.0.0.1#5353", nil
	case "uci get dhcp.@dnsmasq[0].noresolv":
		return "1", nil
	default:
		return "", nil
	}
}

func (m *repairAdGuardChecker) HTTPGet(string) ([]byte, error) {
	return []byte(`{"port":5353}`), nil
}

func (m *repairAdGuardChecker) ReadFile(path string) ([]byte, error) {
	if content, ok := m.content[path]; ok {
		return []byte(content), nil
	}
	return nil, os.ErrNotExist
}

func (m *repairAdGuardChecker) WriteFile(path string, data []byte, _ os.FileMode) error {
	m.files[path] = true
	m.content[path] = string(data)
	return nil
}

func (m *repairAdGuardChecker) RenameFile(oldPath, newPath string) error {
	content, ok := m.content[oldPath]
	if !ok {
		return os.ErrNotExist
	}
	m.content[newPath] = content
	m.files[newPath] = true
	delete(m.content, oldPath)
	delete(m.files, oldPath)
	return nil
}

func (m *repairAdGuardChecker) RemoveFile(path string) error {
	delete(m.files, path)
	delete(m.content, path)
	return nil
}

func (m *repairAdGuardChecker) TCPProbe(string, time.Duration) bool { return true }

func TestRepairAdGuardEndpointRestoresPackageConfig(t *testing.T) {
	checker := &repairAdGuardChecker{
		files:   map[string]bool{"/etc/init.d/adguardhome": true},
		content: map[string]string{},
	}
	svc := uplink.NewAdGuardService(checker)
	app := fiber.New()
	registerAdGuard(NewAPI(app), svc, nil)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/adguard/repair", nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("repair request: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("repair status = %d, body: %s", resp.StatusCode, body)
	}

	var status models.AdGuardDNSStatus
	if err := json.Unmarshal(body, &status); err != nil {
		t.Fatalf("decode repair response: %v", err)
	}
	if !status.ConfigPresent || status.RepairNeeded {
		t.Errorf("repair did not restore a healthy config: %+v", status)
	}
	if checker.content["/etc/adguardhome/adguardhome.yaml"] == "" {
		t.Errorf("package config was not written; calls: %v", checker.calls)
	}
}

var _ uplink.AdGuardChecker = (*repairAdGuardChecker)(nil)
