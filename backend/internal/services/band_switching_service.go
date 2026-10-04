package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	defaultBandSwitchCheckInterval = 10
	defaultDownSwitchThresholdDBm  = -70
	defaultDownSwitchDelaySec      = 30
	defaultUpSwitchThresholdDBm    = -60
	defaultUpSwitchDelaySec        = 60
	defaultMinViableSignalDBm      = -80
	bandSwitchCooldownSec          = 120
)

// bandSwitchGuardFile is a var, not a const, so tests can point the crash guard
// at a temp directory instead of the real /etc/trafo.
var bandSwitchGuardFile = crashGuardDir + "/band-switch-in-progress"

// BandSwitchConfig holds user-configurable parameters for automatic band switching.
type BandSwitchConfig struct {
	Enabled                bool   `json:"enabled"`
	PreferredBand          string `json:"preferred_band"` // "5g" or "2g"
	CheckIntervalSec       int    `json:"check_interval_sec"`
	DownSwitchThresholdDBm int    `json:"down_switch_threshold_dbm"`
	DownSwitchDelaySec     int    `json:"down_switch_delay_sec"`
	UpSwitchThresholdDBm   int    `json:"up_switch_threshold_dbm"`
	UpSwitchDelaySec       int    `json:"up_switch_delay_sec"`
	MinViableSignalDBm     int    `json:"min_viable_signal_dbm"`
}

// BandSwitchStatus holds the real-time monitoring state of the band switcher.
type BandSwitchStatus struct {
	// State: "inactive", "monitoring", "weak_signal", "cooldown", "blocked"
	//
	// "blocked" is reported when a switch the switcher wanted to make was
	// refused because it would have put the uplink and an access point on one
	// PHY. Without it the UI reads "monitoring" and the operator has no way to
	// tell a feature that never fires from one that is working.
	State            string `json:"state"`
	CurrentBand      string `json:"current_band"`
	SignalDBM        int    `json:"signal_dbm"`
	WeakSignalSecs   int    `json:"weak_signal_secs"`
	CooldownSec      int    `json:"cooldown_sec"`
	LastSwitchAt     string `json:"last_switch_at,omitempty"`
	LastSwitchReason string `json:"last_switch_reason,omitempty"`
}

// BandSwitchingService monitors STA signal and switches bands automatically.
type BandSwitchingService struct {
	wifi       *WifiService
	configFile string
	mu         sync.RWMutex
	config     BandSwitchConfig
	status     BandSwitchStatus
	// blockedReason is why the last wanted switch could not be made, or "" when
	// the switcher is free to switch. blockedRadio is the radio that switch
	// wanted, which is what makes the block re-checkable. Read through liveState
	// so the state the UI sees survives the next tick instead of being
	// overwritten by "monitoring", and cleared by refreshBlocked once the layout
	// allows that switch again.
	blockedReason string
	blockedRadio  string
	stopCh        chan struct{}
	stopOnce      sync.Once
}

// NewBandSwitchingService creates a new BandSwitchingService.
func NewBandSwitchingService(wifi *WifiService, configFile string) *BandSwitchingService {
	svc := &BandSwitchingService{
		wifi:       wifi,
		configFile: configFile,
		stopCh:     make(chan struct{}),
		config:     defaultBandSwitchConfig(),
		status:     BandSwitchStatus{State: "inactive"},
	}
	_ = svc.loadConfig()
	return svc
}

func defaultBandSwitchConfig() BandSwitchConfig {
	return BandSwitchConfig{
		Enabled:                false,
		PreferredBand:          "5g",
		CheckIntervalSec:       defaultBandSwitchCheckInterval,
		DownSwitchThresholdDBm: defaultDownSwitchThresholdDBm,
		DownSwitchDelaySec:     defaultDownSwitchDelaySec,
		UpSwitchThresholdDBm:   defaultUpSwitchThresholdDBm,
		UpSwitchDelaySec:       defaultUpSwitchDelaySec,
		MinViableSignalDBm:     defaultMinViableSignalDBm,
	}
}

// GetConfig returns the current band switching configuration.
func (b *BandSwitchingService) GetConfig() BandSwitchConfig {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.config
}

// GetStatus returns the current monitoring status.
func (b *BandSwitchingService) GetStatus() BandSwitchStatus {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.status
}

// SetConfig updates the configuration and persists it.
func (b *BandSwitchingService) SetConfig(cfg BandSwitchConfig) error {
	if cfg.CheckIntervalSec <= 0 {
		cfg.CheckIntervalSec = defaultBandSwitchCheckInterval
	}
	b.mu.Lock()
	b.config = cfg
	b.mu.Unlock()
	return b.saveConfig()
}

// loadConfig reads the persisted config and applies it ONLY if it validates.
//
// The API handler validates before SetConfig, but the file on disk is not
// covered by that: a config written by an older build, hand-edited, or
// truncated to check_interval_sec 0 reached time.NewTicker(0) in Start, which
// PANICS — and Start runs in the backend process, so the API, the WebSocket and
// every other service went down with it. An invalid file now falls back to the
// defaults and says so in the log, which is the same outcome as a device that
// has never saved a config.
func (b *BandSwitchingService) loadConfig() error {
	data, err := os.ReadFile(b.configFile)
	if err != nil {
		return err
	}
	var cfg BandSwitchConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		log.Printf("band-switching: %s is not valid JSON (%v); keeping defaults", b.configFile, err)
		return nil
	}
	if err := ValidateBandSwitchConfig(cfg); err != nil {
		log.Printf("band-switching: %s is invalid (%v); keeping the default config", b.configFile, err)
		return nil
	}
	b.mu.Lock()
	b.config = cfg
	b.mu.Unlock()
	return nil
}

func (b *BandSwitchingService) saveConfig() error {
	b.mu.RLock()
	data, err := json.MarshalIndent(b.config, "", "  ")
	b.mu.RUnlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(b.configFile), 0750); err != nil {
		return err
	}
	return os.WriteFile(b.configFile, data, 0600)
}

// checkInterval is the ticker period, clamped so a bad config can never reach
// time.NewTicker with a zero or negative duration (which panics). Start uses it
// instead of reading the raw field, so the clamp holds for every config source.
func (b *BandSwitchingService) checkInterval() time.Duration {
	b.mu.RLock()
	sec := b.config.CheckIntervalSec
	b.mu.RUnlock()
	if sec <= 0 {
		sec = defaultBandSwitchCheckInterval
	}
	return time.Duration(sec) * time.Second
}

// Start begins the band switching monitor goroutine.
// Must be called once after service creation.
func (b *BandSwitchingService) Start() {
	// Safety: if a crash guard exists from a previous run, log and skip automatic switching.
	if _, err := os.Stat(bandSwitchGuardFile); err == nil {
		log.Printf("band-switching: crash guard found at %s — skipping auto switch; remove manually to re-enable", bandSwitchGuardFile)
	}

	go func() {
		ticker := time.NewTicker(b.checkInterval())
		defer ticker.Stop()

		var (
			weakSignalSecs int
			// upSignalSecs accumulates how long the preferred band has been strong
			// enough to return to. up_switch_delay_sec used to be parsed,
			// validated and persisted, and then never read by any logic.
			upSignalSecs int
			cooldownSec  int
		)

		for {
			select {
			case <-ticker.C:
				b.mu.RLock()
				cfg := b.config
				b.mu.RUnlock()

				if !cfg.Enabled {
					b.mu.Lock()
					b.status = BandSwitchStatus{State: "inactive"}
					b.blockedReason = ""
					b.blockedRadio = ""
					b.mu.Unlock()
					weakSignalSecs = 0
					upSignalSecs = 0
					cooldownSec = 0
					// Reset ticker if interval changed
					ticker.Reset(b.checkInterval())
					continue
				}

				// A block belongs to the radio layout that caused it, so it is
				// re-checked here rather than kept for good: the layout can
				// change under the switcher at any time.
				b.refreshBlocked()

				// Safety: do not switch if crash guard is present.
				if _, err := os.Stat(bandSwitchGuardFile); err == nil {
					b.mu.Lock()
					b.status.State = "inactive"
					b.mu.Unlock()
					continue
				}

				ssid, signalDBM, currentRadio, err := b.wifi.GetSTASignalInfo()
				if err != nil || ssid == "" || currentRadio == "" {
					b.mu.Lock()
					b.status = BandSwitchStatus{State: "inactive"}
					b.mu.Unlock()
					weakSignalSecs = 0
					upSignalSecs = 0
					cooldownSec = 0
					continue
				}

				// Determine if we are on preferred band.
				radios := b.getRadios()
				preferredRadio := b.findRadioByBand(radios, cfg.PreferredBand)
				alternateRadio := b.findAlternateRadio(radios, preferredRadio)
				onPreferredBand := currentRadio == preferredRadio
				if onPreferredBand {
					upSignalSecs = 0
				}

				// Update cooldown.
				if cooldownSec > 0 {
					cooldownSec -= cfg.CheckIntervalSec
					if cooldownSec < 0 {
						cooldownSec = 0
					}
					b.mu.Lock()
					b.status = BandSwitchStatus{
						State:            "cooldown",
						CurrentBand:      b.bandForRadio(radios, currentRadio),
						SignalDBM:        signalDBM,
						CooldownSec:      cooldownSec,
						LastSwitchAt:     b.status.LastSwitchAt,
						LastSwitchReason: b.status.LastSwitchReason,
					}
					b.mu.Unlock()
					continue
				}

				// Down-switch logic: signal too weak → switch to alternate band.
				if signalDBM < cfg.DownSwitchThresholdDBm {
					weakSignalSecs += cfg.CheckIntervalSec
				} else {
					weakSignalSecs = 0
				}

				if weakSignalSecs >= cfg.DownSwitchDelaySec && alternateRadio != "" {
					// Try switching to alternate radio.
					altSignal, found, _ := b.wifi.ScanRadioForSSID(alternateRadio, ssid)
					if found && altSignal >= cfg.MinViableSignalDBm {
						reason := fmt.Sprintf("signal on %s too weak (%d dBm for %ds), switched to %s (%d dBm)",
							b.bandForRadio(radios, currentRadio), signalDBM, weakSignalSecs,
							b.bandForRadio(radios, alternateRadio), altSignal)
						if err := b.doSwitch(alternateRadio, reason); err == nil {
							weakSignalSecs = 0
							upSignalSecs = 0
							cooldownSec = bandSwitchCooldownSec
						}
					}
					// continue is intentional — don't update status after switch
					continue
				}

				// Up-switch logic: if on non-preferred band, check if preferred recovered.
				// up_switch_delay_sec holds the switch back until the preferred band has
				// been strong for that many seconds, so one good scan does not bounce
				// the client straight back onto the band that just failed. 0 means "as
				// soon as the threshold is met", i.e. the behaviour from when this
				// setting was parsed but never used.
				if !onPreferredBand && preferredRadio != "" {
					prefSignal, found, _ := b.wifi.ScanRadioForSSID(preferredRadio, ssid)
					if found && prefSignal >= cfg.UpSwitchThresholdDBm {
						upSignalSecs += cfg.CheckIntervalSec
						if upSignalSecs >= cfg.UpSwitchDelaySec {
							reason := fmt.Sprintf("preferred %s recovered (%d dBm), switching back",
								cfg.PreferredBand, prefSignal)
							if err := b.doSwitch(preferredRadio, reason); err == nil {
								weakSignalSecs = 0
								upSignalSecs = 0
								cooldownSec = bandSwitchCooldownSec
							}
						}
					} else {
						upSignalSecs = 0
					}
				}

				state := b.liveState(weakSignalSecs)
				b.mu.Lock()
				b.status = BandSwitchStatus{
					State:            state,
					CurrentBand:      b.bandForRadio(radios, currentRadio),
					SignalDBM:        signalDBM,
					WeakSignalSecs:   weakSignalSecs,
					LastSwitchAt:     b.status.LastSwitchAt,
					LastSwitchReason: b.status.LastSwitchReason,
				}
				b.mu.Unlock()

				ticker.Reset(b.checkInterval())

			case <-b.stopCh:
				return
			}
		}
	}()
}

// Stop stops the band switching monitor. Safe to call multiple times.
func (b *BandSwitchingService) Stop() {
	b.stopOnce.Do(func() { close(b.stopCh) })
}

// liveState names the state the switcher reports on an ordinary tick.
//
// A blocked switcher keeps reporting "blocked" while the layout still blocks
// it: the block is a property of the radio layout, not of this tick, so the
// next one has to say the same thing or the UI flips back to "monitoring" and
// hides that the feature cannot fire. refreshBlocked is what ends the streak
// once the layout leaves room for the switch.
func (b *BandSwitchingService) liveState(weakSignalSecs int) string {
	b.mu.RLock()
	blocked := b.blockedReason
	b.mu.RUnlock()
	if blocked != "" {
		return "blocked"
	}
	if weakSignalSecs > 0 {
		return "weak_signal"
	}
	return "monitoring"
}

// setBlocked records or clears why the last wanted switch could not be made,
// and on which radio. The status follows immediately so the very next API read
// — the operator refreshing the card, not the following tick — already shows
// it.
func (b *BandSwitchingService) setBlocked(reason, radio string) {
	b.mu.Lock()
	b.blockedReason = reason
	b.blockedRadio = radio
	if reason != "" {
		b.status.State = "blocked"
	}
	b.mu.Unlock()
}

// refreshBlocked drops a recorded block the current radio layout no longer
// justifies.
//
// Latching it for good was wrong once SwitchSTAToRadio stopped refusing every
// layout that carries an access point: the switcher would go on reporting
// "blocked" for a layout it can switch from, which is the card claiming a fault
// the switcher has not observed. Re-reading the layout each tick costs a UCI
// read and only when a block is actually on record.
func (b *BandSwitchingService) refreshBlocked() {
	b.mu.RLock()
	reason, radio := b.blockedReason, b.blockedRadio
	b.mu.RUnlock()
	if reason == "" || radio == "" {
		return
	}
	blocked, err := b.switchBlocked(radio)
	if err != nil {
		// The layout could not be read. Keep what is known rather than
		// dropping a block that may still hold.
		return
	}
	if blocked {
		return
	}
	b.mu.Lock()
	b.blockedReason = ""
	b.blockedRadio = ""
	if b.status.State == "blocked" {
		// The next tick writes the ordinary state; until then the UI must not
		// keep reading a block that no longer exists.
		b.status.State = "monitoring"
	}
	b.mu.Unlock()
	log.Printf("band-switching: %s can host the uplink again; no longer blocked", radio)
}

// switchBlocked reports whether the uplink still cannot move to targetRadio:
// the radio runs an access point, no other radio does, and repeater options do
// not allow the uplink alongside an access point. That is the layout
// WifiService refuses with ErrAPAndSTASameRadio, read here without writing
// anything. A single-radio device never reaches this — it can make no split to
// refuse — so only layouts with an alternate radio are described here.
func (b *BandSwitchingService) switchBlocked(targetRadio string) (bool, error) {
	aps, err := b.wifi.GetAPConfigs()
	if err != nil {
		return false, err
	}
	apOnTarget, apOnOtherRadio := false, false
	for _, ap := range aps {
		if !ap.Enabled || ap.Radio == "" {
			continue
		}
		if ap.Radio == targetRadio {
			apOnTarget = true
		} else {
			apOnOtherRadio = true
		}
	}
	if !apOnTarget {
		return false, nil
	}
	return !apOnOtherRadio && !b.wifi.repeaterAllowAPOnSTARadio(true), nil
}

func (b *BandSwitchingService) doSwitch(targetRadio, reason string) error {
	// Write crash guard before touching wireless config.
	if err := os.MkdirAll(filepath.Dir(bandSwitchGuardFile), 0750); err != nil {
		return err
	}
	if err := os.WriteFile(bandSwitchGuardFile, []byte(reason), 0600); err != nil {
		return err
	}

	err := b.wifi.SwitchSTAToRadio(targetRadio)

	// Remove guard on success; on failure the guard remains to prevent retry loop.
	if err == nil {
		_ = os.Remove(bandSwitchGuardFile)
		b.setBlocked("", "")
		log.Printf("band-switching: %s", reason)
		b.mu.Lock()
		b.status.LastSwitchAt = time.Now().UTC().Format(time.RFC3339)
		b.status.LastSwitchReason = reason
		b.mu.Unlock()
		return nil
	}
	if errors.Is(err, ErrAPAndSTASameRadio) {
		// Refused before any write: nothing was applied and nothing is at
		// risk, so the crash guard must not outlive it. Leaving it would
		// disable automatic band switching for good over a request that was
		// never attempted, until somebody removed the file by hand.
		//
		// The refusal is also reported instead of swallowed. SwitchSTAToRadio
		// reconciles the access point off the target radio rather than
		// refusing (that is what keeps the feature working on a layout with an
		// AP on both radios), so what arrives here is a layout where no split
		// is possible at all. Saying so is the difference between a feature the
		// operator can see is stuck and one that silently never fires.
		_ = os.Remove(bandSwitchGuardFile)
		b.setBlocked(err.Error(), targetRadio)
		log.Printf("band-switching: switch to %s blocked: %v — "+
			"automatic band switching cannot make room on that radio", targetRadio, err)
		b.mu.Lock()
		b.status.LastSwitchReason = err.Error()
		b.mu.Unlock()
		return err
	}
	log.Printf("band-switching: switch failed: %v — guard file left in place", err)
	return err
}

func (b *BandSwitchingService) findRadioByBand(radios []BandRadioInfo, band string) string {
	for _, r := range sortedBandRadios(radios) {
		if r.Band == band {
			return r.Name
		}
	}
	return ""
}

func (b *BandSwitchingService) findAlternateRadio(radios []BandRadioInfo, current string) string {
	for _, r := range sortedBandRadios(radios) {
		if r.Name != current {
			return r.Name
		}
	}
	return ""
}

// sortedBandRadios orders the radio list by name. Both picks above decide
// between candidates by taking the first match, so on a device where two radios
// report the same band an unordered list made the switcher change its mind
// every tick — alternating bands and never settling.
func sortedBandRadios(radios []BandRadioInfo) []BandRadioInfo {
	out := append([]BandRadioInfo(nil), radios...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (b *BandSwitchingService) bandForRadio(radios []BandRadioInfo, name string) string {
	for _, r := range radios {
		if r.Name == name {
			return r.Band
		}
	}
	return name
}

// BandRadioInfo is a minimal radio descriptor used internally by BandSwitchingService.
type BandRadioInfo struct {
	Name string
	Band string
}

// GetRadios returns a slim radio list for band switching logic.
// It wraps WifiService.GetRadios() to avoid import cycles on models.
func (b *BandSwitchingService) getRadios() []BandRadioInfo {
	radios, err := b.wifi.GetRadios()
	if err != nil {
		return nil
	}
	result := make([]BandRadioInfo, len(radios))
	for i, r := range radios {
		result[i] = BandRadioInfo{Name: r.Name, Band: r.Band}
	}
	return result
}
