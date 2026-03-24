package services

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// mockDataUsageRunner is a test double for DataUsageRunner.
type mockDataUsageRunner struct {
	installed    bool
	vnstatOutput []byte
	vnstatErr    error
	files        map[string][]byte
}

func (m *mockDataUsageRunner) IsInstalled() bool { return m.installed }
func (m *mockDataUsageRunner) RunJSON(_ ...string) ([]byte, error) {
	return m.vnstatOutput, m.vnstatErr
}
func (m *mockDataUsageRunner) ReadFile(path string) ([]byte, error) {
	if data, ok := m.files[path]; ok {
		return data, nil
	}
	return nil, os.ErrNotExist
}
func (m *mockDataUsageRunner) WriteFile(path string, data []byte, _ os.FileMode) error {
	if m.files == nil {
		m.files = make(map[string][]byte)
	}
	m.files[path] = data
	return nil
}

func buildVnstatJSON(ifaces []vnstatInterface) []byte {
	root := vnstatRoot{Interfaces: ifaces}
	data, _ := json.Marshal(root)
	return data
}

func TestGetDataStatus_NotInstalled(t *testing.T) {
	svc := NewDataUsageServiceWithRunner(&mockDataUsageRunner{installed: false})
	status, err := svc.GetStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Available {
		t.Error("expected Available=false when vnstat not installed")
	}
	if len(status.Interfaces) != 0 {
		t.Error("expected no interfaces when not installed")
	}
}

func TestGetDataStatus_Installed_ParsesOutput(t *testing.T) {
	now := time.Now()
	iface := vnstatInterface{
		Name: "eth0",
		Traffic: vnstatTraffic{
			Total: vnstatBytes{RX: 1000000, TX: 500000},
			Day: []vnstatDay{
				{
					Date: struct {
						Year  int `json:"year"`
						Month int `json:"month"`
						Day   int `json:"day"`
					}{Year: now.Year(), Month: int(now.Month()), Day: now.Day()},
					RX: 10240,
					TX: 5120,
				},
			},
			Month: []vnstatMonth{
				{
					Date: struct {
						Year  int `json:"year"`
						Month int `json:"month"`
					}{Year: now.Year(), Month: int(now.Month())},
					RX: 512000,
					TX: 256000,
				},
			},
		},
	}
	runner := &mockDataUsageRunner{
		installed:    true,
		vnstatOutput: buildVnstatJSON([]vnstatInterface{iface}),
	}
	svc := NewDataUsageServiceWithRunner(runner)
	status, err := svc.GetStatus()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Available {
		t.Error("expected Available=true")
	}
	if len(status.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(status.Interfaces))
	}
	ifc := status.Interfaces[0]
	if ifc.Name != "eth0" {
		t.Errorf("expected name 'eth0', got %q", ifc.Name)
	}
	if ifc.Label != "Ethernet WAN" {
		t.Errorf("expected label 'Ethernet WAN', got %q", ifc.Label)
	}
	if ifc.Today.RXBytes != 10240 {
		t.Errorf("expected Today.RXBytes=10240, got %d", ifc.Today.RXBytes)
	}
	if ifc.Month.TXBytes != 256000 {
		t.Errorf("expected Month.TXBytes=256000, got %d", ifc.Month.TXBytes)
	}
	if ifc.Total.RXBytes != 1000000 {
		t.Errorf("expected Total.RXBytes=1000000, got %d", ifc.Total.RXBytes)
	}
}

func TestGetBudget_NoFile(t *testing.T) {
	svc := NewDataUsageServiceWithRunner(&mockDataUsageRunner{
		installed: true,
		files:     map[string][]byte{},
	})
	cfg, err := svc.GetBudget()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Budgets) != 0 {
		t.Error("expected empty budgets when file not found")
	}
}

func TestSetAndGetBudget(t *testing.T) {
	runner := &mockDataUsageRunner{installed: true, files: map[string][]byte{}}
	svc := NewDataUsageServiceWithRunner(runner)

	input := models.DataBudgetConfig{
		Budgets: []models.DataBudget{
			{
				Interface:           "wwan0",
				MonthlyLimitBytes:   10737418240,
				WarningThresholdPct: 80,
				ResetDay:            1,
			},
		},
	}
	if err := svc.SetBudget(input); err != nil {
		t.Fatalf("SetBudget: %v", err)
	}

	got, err := svc.GetBudget()
	if err != nil {
		t.Fatalf("GetBudget: %v", err)
	}
	if len(got.Budgets) != 1 {
		t.Fatalf("expected 1 budget, got %d", len(got.Budgets))
	}
	if got.Budgets[0].Interface != "wwan0" {
		t.Errorf("expected interface 'wwan0', got %q", got.Budgets[0].Interface)
	}
	if got.Budgets[0].MonthlyLimitBytes != 10737418240 {
		t.Errorf("unexpected monthly limit: %d", got.Budgets[0].MonthlyLimitBytes)
	}
}

func TestCheckBudgetAlerts_OverThreshold(t *testing.T) {
	now := time.Now()
	iface := vnstatInterface{
		Name: "wwan0",
		Traffic: vnstatTraffic{
			Total: vnstatBytes{RX: 5000000000, TX: 5000000000},
			Month: []vnstatMonth{
				{
					Date: struct {
						Year  int `json:"year"`
						Month int `json:"month"`
					}{Year: now.Year(), Month: int(now.Month())},
					RX: 9000000000, // 9 GB RX
					TX: 500000000,  // 0.5 GB TX → total ~9.5 GB > 80% of 10 GB
				},
			},
		},
	}
	budgetJSON := `{"budgets":[{"interface":"wwan0","monthly_limit_bytes":10737418240,"warning_threshold_pct":80,"reset_day":1}]}`
	runner := &mockDataUsageRunner{
		installed:    true,
		vnstatOutput: buildVnstatJSON([]vnstatInterface{iface}),
		files:        map[string][]byte{dataBudgetConfigPath: []byte(budgetJSON)},
	}
	svc := NewDataUsageServiceWithRunner(runner)
	alerts := svc.CheckBudgetAlerts()
	if len(alerts) == 0 {
		t.Fatal("expected at least one budget alert")
	}
	if !strings.Contains(alerts[0], "WiFi Uplink") {
		t.Errorf("alert should mention interface label, got: %s", alerts[0])
	}
}

func TestCheckBudgetAlerts_UnderThreshold(t *testing.T) {
	now := time.Now()
	iface := vnstatInterface{
		Name: "wwan0",
		Traffic: vnstatTraffic{
			Month: []vnstatMonth{
				{
					Date: struct {
						Year  int `json:"year"`
						Month int `json:"month"`
					}{Year: now.Year(), Month: int(now.Month())},
					RX: 100000000, // 100 MB — well under 10 GB limit
					TX: 50000000,
				},
			},
		},
	}
	budgetJSON := `{"budgets":[{"interface":"wwan0","monthly_limit_bytes":10737418240,"warning_threshold_pct":80,"reset_day":1}]}`
	runner := &mockDataUsageRunner{
		installed:    true,
		vnstatOutput: buildVnstatJSON([]vnstatInterface{iface}),
		files:        map[string][]byte{dataBudgetConfigPath: []byte(budgetJSON)},
	}
	svc := NewDataUsageServiceWithRunner(runner)
	alerts := svc.CheckBudgetAlerts()
	if len(alerts) != 0 {
		t.Errorf("expected no alerts under threshold, got: %v", alerts)
	}
}
