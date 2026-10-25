package wifi

import (
	"fmt"
)

// ValidateBandSwitchConfig checks the automatic band switcher's parameters.
//
// Nothing was validated here either: preferred_band accepted "9g" (no such
// band, so the switcher would never match and quietly do nothing) and the
// signal thresholds accepted any integer.
//
// The thresholds are dBm, which is negative; the bounds below are the useful
// range for a STA association rather than an arbitrary sanity check, and they
// also keep the hysteresis meaningful: up_switch must be less than
// down_switch, otherwise the switcher oscillates between states every tick.
func ValidateBandSwitchConfig(c BandSwitchConfig) error {
	switch c.PreferredBand {
	case "", "2g", "5g":
	default:
		return fmt.Errorf("preferred_band must be \"2g\" or \"5g\", got %q", c.PreferredBand)
	}
	if c.CheckIntervalSec < 1 || c.CheckIntervalSec > 3600 {
		return fmt.Errorf("check_interval_sec must be between 1 and 3600, got %d", c.CheckIntervalSec)
	}
	for _, f := range []struct {
		name  string
		value int
	}{
		{"down_switch_threshold_dbm", c.DownSwitchThresholdDBm},
		{"up_switch_threshold_dbm", c.UpSwitchThresholdDBm},
		{"min_viable_signal_dbm", c.MinViableSignalDBm},
	} {
		// dBm runs from about -127 (noise floor) to 0 (theoretical max).
		if f.value < -127 || f.value > 0 {
			return fmt.Errorf("%s must be a dBm value between -127 and 0, got %d", f.name, f.value)
		}
	}
	for _, f := range []struct {
		name  string
		value int
	}{
		{"down_switch_delay_sec", c.DownSwitchDelaySec},
		{"up_switch_delay_sec", c.UpSwitchDelaySec},
	} {
		if f.value < 0 || f.value > 86400 {
			return fmt.Errorf("%s must be between 0 and 86400 seconds, got %d", f.name, f.value)
		}
	}
	// dBm is negative and a STRONGER signal is a LARGER number. Coming back up
	// must therefore require a better signal than going down did, or the two
	// thresholds coincide and the switcher oscillates on every check. The
	// defaults are -70 to drop and -60 to return.
	if c.UpSwitchThresholdDBm != 0 && c.DownSwitchThresholdDBm != 0 &&
		c.UpSwitchThresholdDBm <= c.DownSwitchThresholdDBm {
		return fmt.Errorf("up_switch_threshold_dbm (%d) must be greater than down_switch_threshold_dbm (%d) "+
			"(a stronger signal, i.e. less negative), otherwise the switcher oscillates on every check",
			c.UpSwitchThresholdDBm, c.DownSwitchThresholdDBm)
	}
	return nil
}
