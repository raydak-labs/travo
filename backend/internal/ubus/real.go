package ubus

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var validUbusPath = regexp.MustCompile(`^[a-zA-Z0-9_.]+$`)

// RealUbus implements the Ubus interface by shelling out to the ubus CLI.
type RealUbus struct{}

// NewRealUbus creates a new RealUbus instance.
func NewRealUbus() *RealUbus {
	return &RealUbus{}
}

// validateUbusArg ensures a ubus path or method contains only safe characters.
func validateUbusArg(name, value string) error {
	if !validUbusPath.MatchString(value) {
		return fmt.Errorf("ubus: invalid %s %q", name, value)
	}
	return nil
}

func (r *RealUbus) Call(path, method string, args map[string]interface{}) (map[string]interface{}, error) {
	if err := validateUbusArg("path", path); err != nil {
		return nil, err
	}
	if err := validateUbusArg("method", method); err != nil {
		return nil, err
	}

	cmdArgs := []string{"call", path, method}
	if args != nil {
		msgJSON, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("ubus: failed to marshal args: %w", err)
		}
		cmdArgs = append(cmdArgs, string(msgJSON))
	}

	out, err := exec.Command("ubus", cmdArgs...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ubus call %s %s: %s", path, method, strings.TrimSpace(string(out)))
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return map[string]interface{}{}, nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, fmt.Errorf("ubus call %s %s: failed to parse output: %w", path, method, err)
	}

	return result, nil
}
