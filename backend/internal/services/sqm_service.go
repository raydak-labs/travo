package services

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

const (
	sqmConfigName          = "sqm"
	sqmDefaultSectionName  = "default"
	sqmSectionTypeQueue    = "queue"
	sqmInitScript          = "/etc/init.d/sqm"
	sqmDefaultQdisc        = "cake"
	sqmDefaultScript       = "piece_of_cake.qos"
	sqmFallbackQdisc       = "fq_codel"
	sqmFallbackScript      = "simple.qos"
	sqmAdvancedHintDefault = "Advanced SQM settings are available in LuCI → Network → SQM QoS."
)

// SQMService provides minimal SQM configuration and apply actions.
type SQMService struct {
	uci uci.UCI
	cmd CommandRunner
}

func NewSQMService(u uci.UCI) *SQMService {
	return &SQMService{uci: u, cmd: &RealCommandRunner{}}
}

func NewSQMServiceWithRunner(u uci.UCI, cmd CommandRunner) *SQMService {
	return &SQMService{uci: u, cmd: cmd}
}

func parseBoolUCI(v string) bool {
	v = strings.TrimSpace(v)
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes") || strings.EqualFold(v, "on")
}

func parseIntUCI(v string) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return i
}

func (s *SQMService) findQueueSection() (string, map[string]string) {
	sections, err := s.uci.GetSections(sqmConfigName)
	if err != nil || len(sections) == 0 {
		return "", nil
	}

	// Prefer the conventional "default" section if present.
	if opts, ok := sections[sqmDefaultSectionName]; ok && opts[".type"] == sqmSectionTypeQueue {
		return sqmDefaultSectionName, opts
	}

	// Deterministic fallback: pick the lexicographically first queue section.
	var names []string
	for name, opts := range sections {
		if opts[".type"] == sqmSectionTypeQueue {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "", nil
	}
	sort.Strings(names)
	name := names[0]
	return name, sections[name]
}

func (s *SQMService) ensureQueueSection() (string, error) {
	section, _ := s.findQueueSection()
	if section != "" {
		return section, nil
	}
	if err := s.uci.AddSection(sqmConfigName, sqmDefaultSectionName, sqmSectionTypeQueue); err != nil {
		return "", fmt.Errorf("creating sqm.%s queue section: %w", sqmDefaultSectionName, err)
	}
	return sqmDefaultSectionName, nil
}

// GetConfig reads the SQM config from UCI. If SQM is not configured, returns defaults.
func (s *SQMService) GetConfig() (models.SQMConfig, error) {
	section, opts := s.findQueueSection()
	cfg := models.SQMConfig{
		Enabled:      false,
		Interface:    "",
		DownloadKbit: 0,
		UploadKbit:   0,
		Qdisc:        sqmDefaultQdisc,
		Script:       sqmDefaultScript,
		AdvancedHint: sqmAdvancedHintDefault,
	}
	if section == "" || opts == nil {
		return cfg, nil
	}
	cfg.DetectedUCIID = section
	cfg.Enabled = parseBoolUCI(opts["enabled"])
	cfg.Interface = strings.TrimSpace(opts["interface"])
	cfg.DownloadKbit = parseIntUCI(opts["download"])
	cfg.UploadKbit = parseIntUCI(opts["upload"])
	if q := strings.TrimSpace(opts["qdisc"]); q != "" {
		cfg.Qdisc = q
	}
	if sc := strings.TrimSpace(opts["script"]); sc != "" {
		cfg.Script = sc
	}
	return cfg, nil
}

func normalizeSQMStrings(cfg models.SQMConfig) models.SQMConfig {
	cfg.Interface = strings.TrimSpace(cfg.Interface)
	cfg.Qdisc = strings.TrimSpace(cfg.Qdisc)
	cfg.Script = strings.TrimSpace(cfg.Script)
	return cfg
}

func validateSQMConfig(cfg models.SQMConfig) error {
	if cfg.Enabled {
		if cfg.Interface == "" {
			return fmt.Errorf("interface is required when SQM is enabled")
		}
		if cfg.DownloadKbit == 0 && cfg.UploadKbit == 0 {
			return fmt.Errorf("set at least one bandwidth (download or upload) when SQM is enabled")
		}
	}
	if cfg.DownloadKbit < 0 || cfg.UploadKbit < 0 {
		return fmt.Errorf("bandwidth must be >= 0")
	}
	if cfg.Qdisc != "" {
		switch cfg.Qdisc {
		case "cake", "fq_codel":
		default:
			return fmt.Errorf("unsupported qdisc %q (supported: cake, fq_codel)", cfg.Qdisc)
		}
	}
	if cfg.Script != "" {
		// Minimal allowlist: keep tight to avoid surprises and typos.
		switch cfg.Script {
		case sqmDefaultScript, "layer_cake.qos", sqmFallbackScript:
		default:
			return fmt.Errorf("unsupported script %q (supported: %s, layer_cake.qos, %s)", cfg.Script, sqmDefaultScript, sqmFallbackScript)
		}
	}
	return nil
}

// SetConfig writes SQM config into UCI but does not restart services.
func (s *SQMService) SetConfig(cfg models.SQMConfig) error {
	cfg = normalizeSQMStrings(cfg)
	if cfg.Qdisc == "" {
		cfg.Qdisc = sqmDefaultQdisc
	}
	if cfg.Script == "" {
		// Keep script aligned to qdisc defaults.
		if cfg.Qdisc == sqmFallbackQdisc {
			cfg.Script = sqmFallbackScript
		} else {
			cfg.Script = sqmDefaultScript
		}
	}
	if err := validateSQMConfig(cfg); err != nil {
		return err
	}

	section, err := s.ensureQueueSection()
	if err != nil {
		return err
	}
	enabled := "0"
	if cfg.Enabled {
		enabled = "1"
	}
	if err := s.uci.Set(sqmConfigName, section, "enabled", enabled); err != nil {
		return err
	}
	if err := s.uci.Set(sqmConfigName, section, "interface", cfg.Interface); err != nil {
		return err
	}
	if err := s.uci.Set(sqmConfigName, section, "download", strconv.Itoa(cfg.DownloadKbit)); err != nil {
		return err
	}
	if err := s.uci.Set(sqmConfigName, section, "upload", strconv.Itoa(cfg.UploadKbit)); err != nil {
		return err
	}
	if err := s.uci.Set(sqmConfigName, section, "qdisc", cfg.Qdisc); err != nil {
		return err
	}
	if err := s.uci.Set(sqmConfigName, section, "script", cfg.Script); err != nil {
		return err
	}
	if err := s.uci.Commit(sqmConfigName); err != nil {
		return err
	}
	return nil
}

// Apply commits UCI (already committed by SetConfig) and restarts sqm.
func (s *SQMService) Apply() (string, error) {
	// Quick sanity: fail clearly when init script is missing.
	if _, err := s.cmd.Run(sqmInitScript, "enabled"); err != nil {
		return "", fmt.Errorf("sqm init script not available (is sqm-scripts installed?)")
	}
	out, err := s.cmd.Run(sqmInitScript, "restart")
	return strings.TrimSpace(string(out)), err
}
