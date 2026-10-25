package network

import (
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// Two rules created in the same millisecond must get different IDs, otherwise
// DeletePortForward (which removes every rule matching the ID) takes out both.
func TestNewPortForwardID_UniqueWithinTheSameMillisecond(t *testing.T) {
	var rules []models.PortForwardRule
	seen := make(map[string]bool, 64)
	for range 64 {
		id := newPortForwardID(rules)
		if id == "" {
			t.Fatal("empty ID")
		}
		if seen[id] {
			t.Fatalf("duplicate port-forward ID %q", id)
		}
		seen[id] = true
		rules = append(rules, models.PortForwardRule{ID: id})
	}
}

// An ID that already exists in the rule set must never be handed out again,
// even if it is generated within the same millisecond.
func TestNewPortForwardID_AvoidsExistingIDs(t *testing.T) {
	existing := []models.PortForwardRule{{ID: newPortForwardID(nil)}}
	if got := newPortForwardID(existing); got == existing[0].ID {
		t.Fatalf("reused existing ID %q", got)
	}
}
