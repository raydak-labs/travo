package firmware

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Validation is the reply of `ubus call system validate_firmware_image`
// (/usr/libexec/validate_firmware_image).
type Validation struct {
	Valid       bool            `json:"valid" doc:"The image passed every check"`
	Forceable   bool            `json:"forceable" doc:"OpenWrt would allow forcing an invalid image; Travo never forces"`
	AllowBackup bool            `json:"allow_backup" doc:"Settings can be kept across this image"`
	Tests       map[string]bool `json:"tests" doc:"Individual checks, e.g. fwtool_device_match (the image is for this board)"`
}

var errShape = errors.New("validate_firmware_image: unexpected reply")

// ParseValidation parses the validator reply. valid, forceable and allow_backup must be present.
func ParseValidation(b []byte) (Validation, error) {
	var raw struct {
		Tests       map[string]bool `json:"tests"`
		Valid       *bool           `json:"valid"`
		Forceable   *bool           `json:"forceable"`
		AllowBackup *bool           `json:"allow_backup"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return Validation{}, fmt.Errorf("validate_firmware_image: %w", err)
	}
	if raw.Valid == nil || raw.Forceable == nil || raw.AllowBackup == nil {
		return Validation{}, errShape
	}
	if raw.Tests == nil {
		raw.Tests = map[string]bool{}
	}
	return Validation{Valid: *raw.Valid, Forceable: *raw.Forceable, AllowBackup: *raw.AllowBackup, Tests: raw.Tests}, nil
}

// DroppedPackages lists packages the user added after the running image was flashed:
// names in /etc/apk/world that /rom/etc/apk/world lacks. A sysupgrade image brings its own
// package set, so these are gone after the flash unless the new image was built with them
// (device-facts §10). Version constraints (`kernel=6.12...`) are stripped.
func DroppedPackages(world, rom []byte) []string {
	inRom := map[string]bool{}
	for _, n := range worldNames(rom) {
		inRom[n] = true
	}
	var out []string
	for _, n := range worldNames(world) {
		if !inRom[n] {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

func worldNames(b []byte) []string {
	var names []string
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if i := strings.IndexAny(line, "=<>~ "); i >= 0 {
			line = line[:i]
		}
		if line != "" {
			names = append(names, line)
		}
	}
	return names
}
