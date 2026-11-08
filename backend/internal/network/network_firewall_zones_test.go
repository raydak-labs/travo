package network

import (
	"slices"
	"testing"

	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// uci.GetSections joins list elements with commas on a device ("wan,wan6,wwan").
func TestGetFirewallZones_SplitsCommaJoinedNetworkList(t *testing.T) {
	u := uci.NewMockUCI()
	for opt, val := range map[string]string{".type": "zone", "network": "wan,wan6,wwan"} {
		if err := u.Set("firewall", "zone_wan", opt, val); err != nil {
			t.Fatal(err)
		}
	}
	svc := New(Deps{UCI: u})

	zones, err := svc.GetFirewallZones()
	if err != nil {
		t.Fatal(err)
	}
	for _, z := range zones {
		if z.Name == "wan" {
			if !slices.Equal(z.Network, []string{"wan", "wan6", "wwan"}) {
				t.Errorf("wan zone networks = %v", z.Network)
			}
			return
		}
	}
	t.Fatal("wan zone not returned")
}
