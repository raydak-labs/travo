package services

import (
	"regexp"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/execx"
	"github.com/openwrt-travel-gui/backend/internal/models"
)

// GetLogs retrieves system logs from logread.
// If service is non-empty, only lines containing that service name (case-insensitive) are returned.
// If level is non-empty, only lines at or above that severity are returned.
func (s *SystemService) GetLogs(service, level string) (models.LogResponse, error) {
	out, err := streamLogTail(execx.Quick, "logread")
	if err != nil {
		return models.LogResponse{}, err
	}
	return parseLogOutput("syslog", out, service, level), nil
}

// GetKernelLogs retrieves kernel logs from dmesg.
func (s *SystemService) GetKernelLogs() (models.LogResponse, error) {
	out, err := streamLogTail(execx.Quick, "dmesg")
	if err != nil {
		return models.LogResponse{}, err
	}
	return parseLogOutput("kernel", out, "", ""), nil
}

// logLevelSeverity maps syslog level names to numeric severity (lower = more severe).
var logLevelSeverity = map[string]int{
	"emerg":   0,
	"alert":   1,
	"crit":    2,
	"err":     3,
	"warning": 4,
	"warn":    4,
	"notice":  5,
	"info":    6,
	"debug":   7,
}

// logLevelPattern finds the severity token of a syslog line wherever it sits.
// Matching the token itself (instead of a fixed column) keeps level filtering
// working for every logread variant: with or without the "dow mon day time
// year" prefix, and with a trailing ':' on the token.
var logLevelPattern = regexp.MustCompile(`\b[a-z][a-z0-9_-]*\.(emerg|alert|crit|err|error|warning|warn|notice|info|debug)\b`)

// extractLevel extracts the syslog level from a log line, independently of the
// column the timestamp occupies. BusyBox logread emits
// "Tue Mar 10 22:00:34 2026 kern.info kernel: ..." but other producers (and
// dmesg) drop the weekday, and a fixed index then yields "" for every line —
// which silently disabled GET /logs?level=… filtering.
// Returns the level string (e.g. "info", "err") or empty string if not found.
func extractLevel(line string) string {
	m := logLevelPattern.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	level := m[1]
	// Aliases that map onto the canonical severity names.
	switch level {
	case "error":
		return "err"
	case "warn":
		return "warning"
	}
	return level
}

func parseLogOutput(source, output, service, level string) models.LogResponse {
	raw := strings.Split(strings.TrimSpace(output), "\n")
	lines := make([]models.LogEntry, 0, len(raw))
	serviceLower := strings.ToLower(service)

	// Resolve minimum severity threshold
	minSeverity := -1
	if level != "" {
		if sev, ok := logLevelSeverity[strings.ToLower(level)]; ok {
			minSeverity = sev
		}
	}

	for _, l := range raw {
		if l == "" {
			continue
		}
		if service != "" && !strings.Contains(strings.ToLower(l), serviceLower) {
			continue
		}
		entryLevel := extractLevel(l)
		if minSeverity >= 0 && entryLevel != "" {
			if sev, ok := logLevelSeverity[entryLevel]; ok && sev > minSeverity {
				continue
			}
		}
		// Normalize "warn" to "warning"
		if entryLevel == "warn" {
			entryLevel = "warning"
		}
		lines = append(lines, models.LogEntry{Line: l, Level: entryLevel})
	}
	return models.LogResponse{
		Source: source,
		Lines:  lines,
		Total:  len(lines),
	}
}
