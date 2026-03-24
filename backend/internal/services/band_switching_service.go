package services

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	defaultBandSwitchCheckInterval       = 10
	defaultDownSwitchThresholdDBm        = -70
	defaultDownSwitchDelaySec            = 30
	defaultUpSwitchThresholdDBm          = -60
	defaultUpSwitchDelaySec              = 60
	defaultMinViableSignalDBm            = -80
	bandSwitchCooldownSec                = 120
	bandSwitchGuardFile                  = "/etc/openwrt-travel-gui/band-switch-in-progress"
)

// BandSwitchConfig holds user-configurable parameters for automatic band switching.
type BandSwitchConfig struct {
	Enabled                bool   `json:"enabled"`
	PreferredBand          string `json:"preferred_band"`            // "5g" or "2g"
	CheckIntervalSec       int    `json:"check_interval_sec"`
	DownSwitchThresholdDBm int    `json:"down_switch_threshold_dbm"`
	DownSwitchDelaySec     int    `json:"down_switch_delay_sec"`
	UpSwitchThresholdDBm   int    `json:"up_switch_threshold_dbm"`
	UpSwitchDelaySec       int    `json:"up_switch_delay_sec"`
	MinViableSignalDBm     int    `json:"min_viable_signal_dbm"`
}

// BandSwitchStatus holds the real-time monitoring state of the band switcher.
type BandSwitchStatus struct {
	// State: "inactive", "monitoring", "weak_signal", "cooldown"
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
	stopCh     chan struct{}
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

func (b *BandSwitchingService) loadConfig() error {
	data, err := os.ReadFile(b.configFile)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return json.Unmarshal(data, &b.config)
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

// Start begins the band switching monitor goroutine.
// Must be called once after service creation.
func (b *BandSwitchingService) Start() {
	// Safety: if a crash guard exists from a previous run, log and skip automatic switching.
	if _, err := os.Stat(bandSwitchGuardFile); err == nil {
		log.Printf("band-switching: crash guard found at %s — skipping auto switch; remove manually to re-enable", bandSwitchGuardFile)
	}

	go func() {
		b.mu.RLock()
		interval := time.Duration(b.config.CheckIntervalSec) * time.Second
		b.mu.RUnlock()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		var (
			weakSignalSecs  int
			cooldownSec     int
			onPreferredBand = true // assume we start on preferred band
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
					b.mu.Unlock()
					weakSignalSecs = 0
					cooldownSec = 0
					// Reset ticker if interval changed
					ticker.Reset(time.Duration(cfg.CheckIntervalSec) * time.Second)
					continue
				}

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
					cooldownSec = 0
					continue
				}

				// Determine if we are on preferred band.
				radios := b.getRadios()
				preferredRadio := b.findRadioByBand(radios, cfg.PreferredBand)
				alternateRadio := b.findAlternateRadio(radios, preferredRadio)
				onPreferredBand = currentRadio == preferredRadio

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
							cooldownSec = bandSwitchCooldownSec
							onPreferredBand = alternateRadio == preferredRadio
						}
					}
					continue
				}

				// Up-switch logic: if on non-preferred band, check if preferred recovered.
				if !onPreferredBand && preferredRadio != "" {
					prefSignal, found, _ := b.wifi.ScanRadioForSSID(preferredRadio, ssid)
					if found && prefSignal >= cfg.UpSwitchThresholdDBm {
						reason := fmt.Sprintf("preferred %s recovered (%d dBm), switching back",
							cfg.PreferredBand, prefSignal)
						if err := b.doSwitch(preferredRadio, reason); err == nil {
							weakSignalSecs = 0
							cooldownSec = bandSwitchCooldownSec
							onPreferredBand = true
						}
					}
				}

				state := "monitoring"
				if weakSignalSecs > 0 {
					state = "weak_signal"
				}
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

				ticker.Reset(time.Duration(cfg.CheckIntervalSec) * time.Second)

			case <-b.stopCh:
				return
			}
		}
	}()
}

// Stop stops the band switching monitor.
func (b *BandSwitchingService) Stop() {
	close(b.stopCh)
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
		log.Printf("band-switching: %s", reason)
		b.mu.Lock()
		b.status.LastSwitchAt = time.Now().UTC().Format(time.RFC3339)
		b.status.LastSwitchReason = reason
		b.mu.Unlock()
	} else {
		log.Printf("band-switching: switch failed: %v — guard file left in place", err)
	}
	return err
}

func (b *BandSwitchingService) findRadioByBand(radios []BandRadioInfo, band string) string {
	for _, r := range radios {
		if r.Band == band {
			return r.Name
		}
	}
	return ""
}

func (b *BandSwitchingService) findAlternateRadio(radios []BandRadioInfo, current string) string {
	for _, r := range radios {
		if r.Name != current {
			return r.Name
		}
	}
	return ""
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
