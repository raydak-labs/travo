package network

import (
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestRunDiagnosticsRejectsOptionLikeTargets(t *testing.T) {
	var ran [][]string
	svc := New(Deps{UCI: uci.NewMockUCI(), Ubus: ubus.NewMockUbus(), Cmd: &platform.FuncCommandRunner{
		RunFunc: func(name string, args ...string) ([]byte, error) {
			ran = append(ran, append([]string{name}, args...))
			return []byte("ok"), nil
		},
	}})

	for _, target := range []string{"-f", "--help", " -f", "", "1.1.1.1;reboot", "a b"} {
		result := svc.RunDiagnostics(models.DiagnosticsRequest{Type: "ping", Target: target})
		if result.Error == "" {
			t.Errorf("target %q must be rejected", target)
		}
	}
	if len(ran) != 0 {
		t.Errorf("no diagnostic command may run for a rejected target, got %v", ran)
	}

	for _, target := range []string{"1.1.1.1", "example.com", "nas.local"} {
		result := svc.RunDiagnostics(models.DiagnosticsRequest{Type: "ping", Target: target})
		if result.Error != "" {
			t.Errorf("target %q must be accepted, got %q", target, result.Error)
		}
	}
	if len(ran) != 3 {
		t.Errorf("expected 3 diagnostic runs, got %v", ran)
	}
}
