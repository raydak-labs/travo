package services

import (
	"encoding/json"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// healthKeyPresence records which optional health keys the stored config
// actually contained, so a missing key can be defaulted while an explicit 0
// ("do not use interval-based failure detection") is preserved.
type healthKeyPresence struct {
	failureInterval  bool
	recoveryInterval bool
}

// healthKeyPresenceOf inspects the raw stored config for the optional health keys.
func healthKeyPresenceOf(data []byte) healthKeyPresence {
	var envelope struct {
		Health json.RawMessage `json:"health"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil || len(envelope.Health) == 0 {
		return healthKeyPresence{}
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Health, &keys); err != nil {
		return healthKeyPresence{}
	}
	_, failure := keys["failure_interval"]
	_, recovery := keys["recovery_interval"]
	return healthKeyPresence{failureInterval: failure, recoveryInterval: recovery}
}

func defaultFailoverHealth() models.FailoverHealthConfig {
	return models.FailoverHealthConfig{
		TrackIPs:         []string{"1.1.1.1", "8.8.8.8"},
		Reliability:      1,
		Count:            1,
		Timeout:          2,
		Interval:         5,
		FailureInterval:  5,
		RecoveryInterval: 5,
		Down:             3,
		Up:               3,
	}
}

// normalizeHealth fills in defaults for unset values. An explicit zero
// FailureInterval / RecoveryInterval is meaningful (mwan3 then does not use
// interval-based failure/recovery detection), so it is preserved.
func normalizeHealth(health models.FailoverHealthConfig) models.FailoverHealthConfig {
	def := defaultFailoverHealth()
	if len(health.TrackIPs) > 0 {
		def.TrackIPs = health.TrackIPs
	}
	if health.Reliability > 0 {
		def.Reliability = health.Reliability
	}
	if health.Count > 0 {
		def.Count = health.Count
	}
	if health.Timeout > 0 {
		def.Timeout = health.Timeout
	}
	if health.Interval > 0 {
		def.Interval = health.Interval
	}
	def.FailureInterval = health.FailureInterval
	def.RecoveryInterval = health.RecoveryInterval
	if health.Down > 0 {
		def.Down = health.Down
	}
	if health.Up > 0 {
		def.Up = health.Up
	}
	return def
}

// normalizeStoredHealth normalizes a config read from disk. Only keys that were
// actually present may carry a meaningful zero; a missing key gets the default.
func normalizeStoredHealth(health models.FailoverHealthConfig, present healthKeyPresence) models.FailoverHealthConfig {
	if !present.failureInterval {
		health.FailureInterval = defaultFailoverHealth().FailureInterval
	}
	if !present.recoveryInterval {
		health.RecoveryInterval = defaultFailoverHealth().RecoveryInterval
	}
	return normalizeHealth(health)
}
