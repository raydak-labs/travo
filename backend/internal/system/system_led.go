package system

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/openwrt-travel-gui/backend/internal/execx"
	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
)

// GetLEDStatus returns the current stealth mode state by checking LED brightness.
func (s *Service) GetLEDStatus() models.LEDStatus {
	leds := listLEDs()
	allOff := len(leds) > 0
	var ledInfos []models.LEDInfo
	for _, led := range leds {
		b, err := os.ReadFile(filepath.Join("/sys/class/leds", led, "brightness"))
		brightness := 0
		if err == nil {
			val := strings.TrimSpace(string(b))
			brightness, _ = strconv.Atoi(val)
			if val != "0" {
				allOff = false
			}
		}
		ledInfos = append(ledInfos, models.LEDInfo{
			Name:       led,
			Brightness: brightness,
		})
	}
	return models.LEDStatus{
		StealthMode: allOff,
		LEDCount:    len(leds),
		LEDs:        ledInfos,
	}
}

// SetLEDStealthMode turns all LEDs off (stealth) or restores them.
func (s *Service) SetLEDStealthMode(stealth bool) error {
	leds := listLEDs()
	for _, led := range leds {
		path := filepath.Join("/sys/class/leds", led, "brightness")
		val := "255"
		if stealth {
			val = "0"
		}
		if err := os.WriteFile(path, []byte(val), 0644); err != nil {
			return err
		}
	}
	return nil
}

func listLEDs() []string {
	entries, err := os.ReadDir("/sys/class/leds")
	if err != nil {
		return nil
	}
	var leds []string
	for _, e := range entries {
		leds = append(leds, e.Name())
	}
	return leds
}

const ledCronTag = "# openwrt-travel-gui-led-schedule"

// ledCrontabPath is busybox crond's per-user crontab. It is the ONLY file it
// reads (it is built with -c /etc/crontabs), which is also why the WiFi schedule
// targets the same file.
const ledCrontabPath = "/etc/crontabs/root"

// GetLEDSchedule reads the LED stealth schedule from crontab.
func (s *Service) GetLEDSchedule() models.LEDSchedule {
	data, err := os.ReadFile("/etc/crontabs/root")
	if err != nil {
		return models.LEDSchedule{}
	}
	schedule := models.LEDSchedule{}
	for line := range strings.SplitSeq(string(data), "\n") {
		if !strings.Contains(line, ledCronTag) {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 5 {
			continue
		}
		minute, hour := parts[0], parts[1]
		timeStr := fmt.Sprintf("%s:%s", hour, minute)
		if strings.Contains(line, "brightness-off") {
			schedule.OffTime = timeStr
			schedule.Enabled = true
		} else if strings.Contains(line, "brightness-on") {
			schedule.OnTime = timeStr
			schedule.Enabled = true
		}
	}
	return schedule
}

// SetLEDSchedule writes or removes LED schedule cron entries.
//
// Shares crontabMu with the WiFi schedule: both rewrite /etc/crontabs/root
// while preserving the lines they do not own, so a concurrent pair would drop
// one another's entries.
func (s *Service) SetLEDSchedule(schedule models.LEDSchedule) error {
	platform.CrontabMu.
		Lock()
	defer platform.CrontabMu.Unlock()

	data, _ := os.ReadFile(ledCrontabPath)
	var lines []string
	for line := range strings.SplitSeq(string(data), "\n") {
		if line == "" || strings.Contains(line, ledCronTag) {
			continue
		}
		lines = append(lines, line)
	}
	if schedule.Enabled && schedule.OffTime != "" && schedule.OnTime != "" {
		// Times are formatted into a root crontab line, so they must be strict
		// HH:MM — a newline would inject a second attacker-controlled entry.
		if err := platform.ValidateHHMM(schedule.OffTime); err != nil {
			return fmt.Errorf("off_time: %w", err)
		}
		if err := platform.ValidateHHMM(schedule.OnTime); err != nil {
			return fmt.Errorf("on_time: %w", err)
		}
		offParts := strings.SplitN(schedule.OffTime, ":", 2)
		onParts := strings.SplitN(schedule.OnTime, ":", 2)
		ledScript := "for f in /sys/class/leds/*/brightness; do echo %s > $f; done"
		offLine := fmt.Sprintf("%s %s * * * %s %s", offParts[1], offParts[0], fmt.Sprintf(ledScript, "0"), ledCronTag+" brightness-off")
		onLine := fmt.Sprintf("%s %s * * * %s %s", onParts[1], onParts[0], fmt.Sprintf(ledScript, "255"), ledCronTag+" brightness-on")
		lines = append(lines, offLine, onLine)
	}
	lines = append(lines, "")
	if err := platform.WriteFileAtomic(ledCrontabPath, []byte(strings.Join(lines, "\n"))); err != nil {
		return fmt.Errorf("writing crontab: %w", err)
	}
	_ = execx.Run(execx.Quick, "/etc/init.d/cron", "restart")
	return nil
}
