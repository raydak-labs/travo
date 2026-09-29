package uci

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/execx"
)

var validIdentifier = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
var validSectionType = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`) // OpenWrt uses wifi-iface, wifi-device, etc.
// validSectionNameForList allows named sections (zone_wan) and anonymous (@zone[0]) for firewall etc.
var validSectionNameForList = regexp.MustCompile(`^([a-zA-Z0-9_]+|@[a-zA-Z0-9_]+\[\d+\])$`)

// validListValue allows identifiers + hyphens + dots + slashes + colons for IPs/CIDRs/interface names.
var validListValue = regexp.MustCompile(`^[a-zA-Z0-9_.:/+-]+$`)

// RealUCI implements the UCI interface by shelling out to the uci CLI.
type RealUCI struct{}

// NewRealUCI creates a new RealUCI instance.
func NewRealUCI() *RealUCI {
	return &RealUCI{}
}

// validateIdentifier ensures a UCI config/section/option name contains only safe characters.
func validateIdentifier(name, value string) error {
	if !validIdentifier.MatchString(value) {
		return fmt.Errorf("uci: invalid %s %q", name, value)
	}
	return nil
}

// parseShowOutput parses the output of `uci show config.section` into a map of option→value.
// Lines in format "config.section.option='value'" are parsed; section type lines are skipped.
func parseShowOutput(output string) map[string]string {
	result := make(map[string]string)
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		before, after, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key := before
		val := after

		// Extract option name: key is "config.section.option"
		parts := strings.SplitN(key, ".", 3)
		if len(parts) < 3 {
			// Section type line (e.g. "network.wan=interface"), skip
			continue
		}
		option := parts[2]

		// Strip surrounding single quotes
		// A list option is printed on one line as option='a' 'b' 'c', a scalar
		// as option='a' (or bare). Normalising through SplitUciValue turns a
		// list into the comma-joined form callers already split on, instead of
		// leaving the embedded quotes (a' 'b) in the value.
		val = strings.Join(SplitUciValue(val), ",")

		result[option] = val
	}
	return result
}

func (r *RealUCI) Get(config, section, option string) (string, error) {
	if err := validateIdentifier("config", config); err != nil {
		return "", err
	}
	if err := validateIdentifier("section", section); err != nil {
		return "", err
	}
	if err := validateIdentifier("option", option); err != nil {
		return "", err
	}

	key := fmt.Sprintf("%s.%s.%s", config, section, option)
	out, err := execx.CombinedOutput(execx.Quick, "uci", "get", key)
	if err != nil {
		return "", fmt.Errorf("uci get %s: %s", key, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func (r *RealUCI) Set(config, section, option, value string) error {
	if err := validateIdentifier("config", config); err != nil {
		return err
	}
	if err := validateIdentifier("section", section); err != nil {
		return err
	}
	if err := validateIdentifier("option", option); err != nil {
		return err
	}

	arg := fmt.Sprintf("%s.%s.%s=%s", config, section, option, value)
	out, err := execx.CombinedOutput(execx.Quick, "uci", "set", arg)
	if err != nil {
		return fmt.Errorf("uci set %s.%s.%s: %s", config, section, option, strings.TrimSpace(string(out)))
	}
	return nil
}

func (r *RealUCI) GetAll(config, section string) (map[string]string, error) {
	if err := validateIdentifier("config", config); err != nil {
		return nil, err
	}
	if err := validateIdentifier("section", section); err != nil {
		return nil, err
	}

	key := fmt.Sprintf("%s.%s", config, section)
	out, err := execx.CombinedOutput(execx.Quick, "uci", "show", key)
	if err != nil {
		return nil, fmt.Errorf("uci show %s: %s", key, strings.TrimSpace(string(out)))
	}

	return parseShowOutput(string(out)), nil
}

func (r *RealUCI) Commit(config string) error {
	if err := validateIdentifier("config", config); err != nil {
		return err
	}

	out, err := execx.CombinedOutput(execx.Quick, "uci", "commit", config)
	if err != nil {
		return fmt.Errorf("uci commit %s: %s", config, strings.TrimSpace(string(out)))
	}
	return nil
}

func (r *RealUCI) AddSection(config, section, stype string) error {
	if err := validateIdentifier("config", config); err != nil {
		return err
	}
	if err := validateIdentifier("section", section); err != nil {
		return err
	}
	if !validSectionType.MatchString(stype) {
		return fmt.Errorf("uci: invalid stype %q", stype)
	}

	// "uci set config.section=stype" creates a named section of the given type
	arg := fmt.Sprintf("%s.%s=%s", config, section, stype)
	out, err := execx.CombinedOutput(execx.Quick, "uci", "set", arg)
	if err != nil {
		return fmt.Errorf("uci add section %s.%s: %s", config, section, strings.TrimSpace(string(out)))
	}
	return nil
}

// AddList appends a value to a UCI list option (e.g. firewall zone network list).
// Section may be a named section (zone_wan) or anonymous (@zone[0]).
func (r *RealUCI) AddList(config, section, option, value string) error {
	if err := validateIdentifier("config", config); err != nil {
		return err
	}
	if !validSectionNameForList.MatchString(section) {
		return fmt.Errorf("uci: invalid section name for add_list %q", section)
	}
	if err := validateIdentifier("option", option); err != nil {
		return err
	}
	if !validListValue.MatchString(value) {
		return fmt.Errorf("uci: invalid value for add_list %q", value)
	}
	arg := fmt.Sprintf("%s.%s.%s=%s", config, section, option, value)
	out, err := execx.CombinedOutput(execx.Quick, "uci", "add_list", arg)
	if err != nil {
		return fmt.Errorf("uci add_list %s: %s", arg, strings.TrimSpace(string(out)))
	}
	return nil
}

func (r *RealUCI) DeleteOption(config, section, option string) error {
	if err := validateIdentifier("config", config); err != nil {
		return err
	}
	if err := validateIdentifier("section", section); err != nil {
		return err
	}
	if err := validateIdentifier("option", option); err != nil {
		return err
	}
	key := fmt.Sprintf("%s.%s.%s", config, section, option)
	_ = execx.Run(execx.Quick, "uci", "-q", "delete", key)
	return nil
}

func (r *RealUCI) DeleteSection(config, section string) error {
	if err := validateIdentifier("config", config); err != nil {
		return err
	}
	if err := validateIdentifier("section", section); err != nil {
		return err
	}

	key := fmt.Sprintf("%s.%s", config, section)
	out, err := execx.CombinedOutput(execx.Quick, "uci", "delete", key)
	if err != nil {
		return fmt.Errorf("uci delete %s: %s", key, strings.TrimSpace(string(out)))
	}
	return nil
}

// SplitUciValue splits the value of a `uci show` line into its elements.
//
// uci prints a list option on a single line as option='a' 'b' 'c' and a scalar
// as option='a', or unquoted (option=abc). Trimming only the outer quotes turns
// the first form into the single bogus value "a' 'b", which then defeats every
// caller that splits on commas or looks for one exact element.
func SplitUciValue(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if !strings.HasPrefix(value, "'") {
		return []string{value}
	}
	var out []string
	rest := value
	for {
		start := strings.Index(rest, "'")
		if start < 0 {
			break
		}
		end := strings.Index(rest[start+1:], "'")
		if end < 0 {
			break
		}
		out = append(out, rest[start+1:start+1+end])
		rest = rest[start+1+end+1:]
	}
	if len(out) == 0 {
		// Unbalanced quotes: fall back to the old behaviour rather than drop it.
		return []string{strings.Trim(value, "'")}
	}
	return out
}

// parseShowConfigOutput parses `uci show <config>` output into a map of
// section → options.
func parseShowConfigOutput(output string) map[string]map[string]string {
	result := make(map[string]map[string]string)
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		before, after, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key := before
		// A list option is printed on one line as option='a' 'b' 'c' and a scalar
		// as option='a'. Normalising through SplitUciValue turns the list into
		// the comma-joined form callers already split on, instead of leaving the
		// embedded quotes (a' 'b) in the value.
		val := strings.Join(SplitUciValue(after), ",")

		parts := strings.SplitN(key, ".", 3)
		if len(parts) == 2 {
			// Section type line: config.section=type
			section := parts[1]
			if result[section] == nil {
				result[section] = make(map[string]string)
			}
			result[section][".type"] = val
		} else if len(parts) == 3 {
			// Option line: config.section.option=value
			section := parts[1]
			option := parts[2]
			if result[section] == nil {
				result[section] = make(map[string]string)
			}
			result[section][option] = val
		}
	}
	return result
}

// uciShowConfig runs `uci show <config>`. It is a package-level seam so tests
// can exercise GetSections error handling without the uci binary.
var uciShowConfig = func(config string) ([]byte, error) {
	return execx.CombinedOutput(execx.Quick, "uci", "show", config)
}

// isMissingUCIConfig reports whether a failed `uci show` means "this config
// package is not installed" (uci exits non-zero with "Entry not found").
// That is the one failure that legitimately means "no sections"; every other
// failure (timeout, unreadable file, lock contention) must surface, otherwise
// callers cannot tell a broken system from an empty config.
func isMissingUCIConfig(out string) bool {
	lower := strings.ToLower(out)
	return strings.Contains(lower, "entry not found") ||
		strings.Contains(lower, "no such file") ||
		strings.Contains(lower, "not found")
}

func (r *RealUCI) GetSections(config string) (map[string]map[string]string, error) {
	if err := validateIdentifier("config", config); err != nil {
		return nil, err
	}

	out, err := uciShowConfig(config)
	if err != nil {
		if isMissingUCIConfig(string(out)) {
			return map[string]map[string]string{}, nil
		}
		return nil, fmt.Errorf("uci show %s: %s", config, strings.TrimSpace(string(out)))
	}
	return parseShowConfigOutput(string(out)), nil
}

// Revert discards staged (uncommitted) changes for a config. The uci CLI keeps
// its delta in /tmp/.uci/<config>/changes, which is process-global: a staged
// write that is abandoned would be committed by a later, unrelated
// `uci commit <config>`. Callers use this to roll back a failed write sequence.
func (r *RealUCI) Revert(config string) error {
	if err := validateIdentifier("config", config); err != nil {
		return err
	}

	out, err := execx.CombinedOutput(execx.Quick, "uci", "revert", config)
	if err != nil {
		return fmt.Errorf("uci revert %s: %s", config, strings.TrimSpace(string(out)))
	}
	return nil
}
