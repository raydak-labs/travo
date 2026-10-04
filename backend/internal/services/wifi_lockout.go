package services

import (
	"errors"
	"fmt"

	"github.com/openwrt-travel-gui/backend/internal/ubus"
)

// ErrLockoutRefused refuses a wireless change that would leave the operator who
// asked for it with no access point to reconnect through.
//
// It is the one safety error in this package that is about the CALLER rather
// than about the config: the same request from a wired console is fine, and the
// same request with the acknowledgement set is fine. That is deliberate — a
// router that cannot be reached after a change it applied correctly cannot
// explain itself, and the operator has no way back short of physical access.
var ErrLockoutRefused = errors.New("refusing to remove the access point you are connected " +
	"through: connect over Ethernet first, or resend with acknowledge_lockout to accept " +
	"losing WiFi access")

// LockoutErrorCode is the stable machine-readable code the HTTP layer puts next
// to the lockout message. The frontend keys off this, never off the message
// text, so the wording above can be reworded without breaking clients.
const LockoutErrorCode = "wifi_lockout_risk"

// LockoutRequest carries the caller's identity and their acknowledgement into
// every guarded mutator. An empty ClientIP means "no caller context" (a
// background flow, a test): the guard then cannot see a WiFi caller and lets
// the change through, which is the only behaviour that keeps non-HTTP callers
// working.
type LockoutRequest struct {
	// ClientIP is the address the mutating request came from.
	ClientIP string
	// AcknowledgeLockout is the operator's explicit "yes, do it anyway". Set by
	// the frontend only after a dialog that requires ticking a box has been
	// answered.
	AcknowledgeLockout bool
}

// wifiCallerIP reports whether clientIP is on a WiFi interface, reusing
// NetworkService.GetConnectionMethod rather than a second notion of "who is the
// caller": two classifiers would drift, and the one on the connection-method
// endpoint is what the operator's own UI already shows them.
func wifiCallerIP(ub ubus.Ubus, clientIP string) bool {
	switch classifyClientConnection(ub, clientIP).Method {
	case "wifi-client", "wifi-ap":
		return true
	default:
		return false
	}
}

// guardLockoutExcluding is the shape every endpoint that turns something OFF
// uses: work out what the config would look like after the change, then run the
// one rule on it. Empty excludes mean "nothing is being removed", which is what
// the endpoints that keep an access point enabled pass.
func (w *WifiService) guardLockoutExcluding(
	req LockoutRequest, excludeSection, excludeRadio string,
) error {
	if req.AcknowledgeLockout || req.ClientIP == "" {
		return nil
	}
	remains, err := w.enabledAPRemains(excludeSection, excludeRadio)
	if err != nil {
		// Fail closed: an unreadable config means the guard cannot prove the
		// operator keeps an access point, so it refuses and says why.
		return err
	}
	return w.guardLockout(req, remains)
}

// guardLockout is the ONE rule every guarded mutator runs. Call it with
// enabledAPRemains — "after this change, is at least one enabled mode=ap
// wifi-iface left on any radio" — and it either returns nil (proceed) or
// ErrLockoutRefused (write nothing, stage nothing, answer 409).
//
// It must be called BEFORE the mutation starts, not inside it, so a refusal
// cannot leave a staged UCI delta, a committed config, or an apply session
// behind.
//
// Why "no enabled AP left anywhere" and not "the caller's own AP goes away":
//   - it is conservative in the safe direction — the only false positive is an
//     operator on WiFi who is turning off the LAST access point while also on
//     WiFi, and they are told exactly that and can acknowledge it;
//   - it cannot strand anyone it does not refuse, because it only fires when the
//     result has no access point at all;
//   - it does not fire when the operator is on the other radio and that radio's
//     AP stays enabled, which is the everyday case ("turn 5G off while I am on
//     2.4G"), so a narrower rule would add a second thing to keep in agreement
//     for no additional safety.
func (w *WifiService) guardLockout(req LockoutRequest, enabledAPRemains bool) error {
	if req.AcknowledgeLockout || req.ClientIP == "" {
		return nil
	}
	if enabledAPRemains {
		return nil
	}
	if !wifiCallerIP(w.ubus, req.ClientIP) {
		return nil
	}
	return ErrLockoutRefused
}

// enabledAPRemains reports whether at least one enabled mode=ap wifi-iface
// would still be up once the described change is applied. excludeSection drops
// one wifi-iface (an AP being disabled), excludeRadio drops every AP on one
// radio (a radio being switched off). Empty strings exclude nothing.
//
// A wifi-iface on a radio with disabled=1 counts for nothing: netifd will not
// bring it up, so counting it would let the last usable access point look
// available.
func (w *WifiService) enabledAPRemains(excludeSection, excludeRadio string) (bool, error) {
	sections, err := w.uci.GetSections("wireless")
	if err != nil {
		return false, fmt.Errorf("lockout guard could not read the wireless config: %w", err)
	}
	for _, section := range activeIfaces(sections, "ap") {
		if section == excludeSection {
			continue
		}
		radio := sections[section]["device"]
		if radio == excludeRadio {
			continue
		}
		radioOpts, err := w.uci.GetAll("wireless", radio)
		if err != nil {
			return false, fmt.Errorf("lockout guard could not read wireless.%s: %w", radio, err)
		}
		if radioOpts["disabled"] == "1" {
			continue
		}
		return true, nil
	}
	return false, nil
}
