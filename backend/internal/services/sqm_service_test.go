package services

import (
	"errors"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestSQMService_GetConfig_DefaultsWhenMissing(t *testing.T) {
	u := uci.NewMockUCI()
	s := NewSQMServiceWithRunner(u, &MockCommandRunner{})

	cfg, err := s.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig error: %v", err)
	}
	if cfg.Enabled {
		t.Fatalf("expected enabled=false")
	}
	if cfg.Qdisc == "" || cfg.Script == "" {
		t.Fatalf("expected default qdisc/script")
	}
}

func TestSQMService_SetConfig_CreatesSectionAndCommits(t *testing.T) {
	u := uci.NewMockUCI()
	s := NewSQMServiceWithRunner(u, &MockCommandRunner{})

	err := s.SetConfig(models.SQMConfig{
		Enabled:      true,
		Interface:    "pppoe-wan",
		DownloadKbit: 20000,
		UploadKbit:   5000,
		Qdisc:        "cake",
		Script:       "piece_of_cake.qos",
	})
	if err != nil {
		t.Fatalf("SetConfig error: %v", err)
	}

	// Verify UCI values exist.
	sections, _ := u.GetSections("sqm")
	if len(sections) == 0 {
		t.Fatalf("expected sqm section to exist")
	}
	found := false
	for _, opts := range sections {
		if opts[".type"] == "queue" {
			found = true
			if opts["interface"] != "pppoe-wan" {
				t.Fatalf("expected interface set, got %q", opts["interface"])
			}
		}
	}
	if !found {
		t.Fatalf("expected a queue section")
	}
}

func TestSQMService_SetConfig_Validates(t *testing.T) {
	u := uci.NewMockUCI()
	s := NewSQMServiceWithRunner(u, &MockCommandRunner{})

	if err := s.SetConfig(models.SQMConfig{Enabled: true}); err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestSQMService_Apply_RestartsInitScript(t *testing.T) {
	u := uci.NewMockUCI()
	var gotName string
	var gotArgs []string
	cmd := &MockCommandRunner{
		RunFunc: func(name string, args ...string) ([]byte, error) {
			gotName = name
			gotArgs = args
			if name == "/etc/init.d/sqm" && len(args) == 1 && args[0] == "enabled" {
				return []byte("enabled"), nil
			}
			return []byte("ok"), nil
		},
	}
	s := NewSQMServiceWithRunner(u, cmd)

	out, err := s.Apply()
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if out != "ok" {
		t.Fatalf("expected output ok, got %q", out)
	}
	if gotName != "/etc/init.d/sqm" {
		t.Fatalf("expected init script, got %q", gotName)
	}
	if len(gotArgs) != 1 || gotArgs[0] != "restart" {
		t.Fatalf("expected restart args, got %#v", gotArgs)
	}

	cmd.RunFunc = func(name string, args ...string) ([]byte, error) {
		if name == "/etc/init.d/sqm" && len(args) == 1 && args[0] == "enabled" {
			return []byte("enabled"), nil
		}
		return []byte("fail"), errors.New("boom")
	}
	_, err = s.Apply()
	if err == nil {
		t.Fatalf("expected error")
	}
}
