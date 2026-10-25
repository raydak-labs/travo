package failover

import (
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

func TestNormalizeHealthPreservesExplicitZeroIntervals(t *testing.T) {
	t.Parallel()

	health := normalizeHealth(models.FailoverHealthConfig{
		TrackIPs:         []string{"1.1.1.1"},
		Reliability:      2,
		Count:            2,
		Timeout:          3,
		Interval:         10,
		FailureInterval:  0,
		RecoveryInterval: 0,
		Down:             4,
		Up:               4,
	})
	if health.FailureInterval != 0 {
		t.Errorf("explicit failure_interval 0 must be preserved, got %d", health.FailureInterval)
	}
	if health.RecoveryInterval != 0 {
		t.Errorf("explicit recovery_interval 0 must be preserved, got %d", health.RecoveryInterval)
	}
}

func TestNormalizeStoredHealthDefaultsMissingKeys(t *testing.T) {
	t.Parallel()

	def := defaultFailoverHealth()
	got := normalizeStoredHealth(models.FailoverHealthConfig{Interval: 10}, healthKeyPresence{})
	if got.FailureInterval != def.FailureInterval {
		t.Errorf("missing failure_interval must fall back to the default, got %d", got.FailureInterval)
	}
	if got.RecoveryInterval != def.RecoveryInterval {
		t.Errorf("missing recovery_interval must fall back to the default, got %d", got.RecoveryInterval)
	}

	got = normalizeStoredHealth(models.FailoverHealthConfig{Interval: 10, FailureInterval: 0}, healthKeyPresence{failureInterval: true})
	if got.FailureInterval != 0 {
		t.Errorf("stored failure_interval 0 must be preserved, got %d", got.FailureInterval)
	}
}
