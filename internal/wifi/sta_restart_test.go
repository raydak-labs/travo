package wifi

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/lockout"
)

// The captured layout (STA on radio0, AP on radio1) never drops an AP: nothing to check,
// not even for a caller that cannot be classified.
func TestSTARestartWithAPOnOtherRadioIsAllowed(t *testing.T) {
	s, _, _ := apService(t, apUbus(t))
	if err := s.CheckSTARestart(context.Background(), netip.MustParseAddr("192.168.1.250"), false); err != nil {
		t.Fatal(err)
	}
}

// sharedRadioUbus moves the captured radio1 interfaces onto radio0, next to the STA: the
// layout of an AXT1800 whose AP shares the uplink's band.
func sharedRadioUbus(t *testing.T) *fakeUbus {
	t.Helper()
	u := apUbus(t)
	key := ubusKey{object: "network.wireless", method: "status"}
	var radios map[string]map[string]json.RawMessage
	if err := json.Unmarshal(u.replies[key], &radios); err != nil {
		t.Fatal(err)
	}
	var sta, ap []json.RawMessage
	if json.Unmarshal(radios["radio0"]["interfaces"], &sta) != nil || json.Unmarshal(radios["radio1"]["interfaces"], &ap) != nil {
		t.Fatal("interfaces")
	}
	radios["radio0"]["interfaces"], _ = json.Marshal(append(sta, ap...))
	radios["radio1"]["interfaces"] = json.RawMessage("[]")
	u.replies[key], _ = json.Marshal(radios)
	return u
}

func TestSTARestartWithAPOnSTARadio(t *testing.T) {
	s, _, _ := apService(t, sharedRadioUbus(t))
	ctx := context.Background()
	// The captured AP client: its only AP drops during the restart.
	station := netip.MustParseAddr("192.168.1.151")
	if err := s.CheckSTARestart(ctx, station, false); !errors.Is(err, lockout.ErrLockoutRisk) {
		t.Fatalf("wifi caller on the shared radio: %v", err)
	}
	if err := s.CheckSTARestart(ctx, station, true); err != nil {
		t.Fatalf("acknowledged: %v", err)
	}
	// The wired neighbour is not a station of any AP: Ethernet is never blocked.
	if err := s.CheckSTARestart(ctx, netip.MustParseAddr("192.168.1.2"), false); err != nil {
		t.Fatalf("ethernet caller: %v", err)
	}
	// An address with no neighbour entry is unknown: fail closed.
	if err := s.CheckSTARestart(ctx, netip.MustParseAddr("192.168.1.251"), false); !errors.Is(err, lockout.ErrLockoutRisk) {
		t.Fatalf("unknown caller: %v", err)
	}
}

func TestSTARestartUnreadableIsUnavailable(t *testing.T) {
	u := apUbus(t)
	delete(u.replies, ubusKey{object: "network.wireless", method: "status"})
	s, _, _ := apService(t, u)
	if err := s.CheckSTARestart(context.Background(), netip.Addr{}, false); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}
