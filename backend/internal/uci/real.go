package uci

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
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
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		eqIdx := strings.Index(line, "=")
		if eqIdx < 0 {
			continue
		}
		key := line[:eqIdx]
		val := line[eqIdx+1:]

		// Extract option name: key is "config.section.option"
		parts := strings.SplitN(key, ".", 3)
		if len(parts) < 3 {
			// Section type line (e.g. "network.wan=interface"), skip
			continue
		}
		option := parts[2]

		// Strip surrounding single quotes
		val = strings.TrimPrefix(val, "'")
		val = strings.TrimSuffix(val, "'")

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
	out, err := exec.Command("uci", "get", key).CombinedOutput()
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
	out, err := exec.Command("uci", "set", arg).CombinedOutput()
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
	out, err := exec.Command("uci", "show", key).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("uci show %s: %s", key, strings.TrimSpace(string(out)))
	}

	return parseShowOutput(string(out)), nil
}

func (r *RealUCI) Commit(config string) error {
	if err := validateIdentifier("config", config); err != nil {
		return err
	}

	out, err := exec.Command("uci", "commit", config).CombinedOutput()
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
	out, err := exec.Command("uci", "set", arg).CombinedOutput()
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
	out, err := exec.Command("uci", "add_list", arg).CombinedOutput()
	if err != nil {
		return fmt.Errorf("uci add_list %s: %s", arg, strings.TrimSpace(string(out)))
	}
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
	out, err := exec.Command("uci", "delete", key).CombinedOutput()
	if err != nil {
		return fmt.Errorf("uci delete %s: %s", key, strings.TrimSpace(string(out)))
	}
	return nil
}

// parseShowConfigOutput parses `uci show <config>` output into a map of section → options.
func parseShowConfigOutput(output string) map[string]map[string]string {
	result := make(map[string]map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		eqIdx := strings.Index(line, "=")
		if eqIdx < 0 {
			continue
		}
		key := line[:eqIdx]
		val := line[eqIdx+1:]

		val = strings.TrimPrefix(val, "'")
		val = strings.TrimSuffix(val, "'")

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

func (r *RealUCI) GetSections(config string) (map[string]map[string]string, error) {
	if err := validateIdentifier("config", config); err != nil {
		return nil, err
	}

	out, err := exec.Command("uci", "show", config).CombinedOutput()
	if err != nil {
		return map[string]map[string]string{}, nil
	}
	return parseShowConfigOutput(string(out)), nil
}
