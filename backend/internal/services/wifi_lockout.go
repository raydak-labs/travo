package services

import (
	"errors"
	"fmt"
)

// ErrLockoutRefused refuses a wireless change that would leave the operator who
// asked for it with no access point to reconnect through.
//
// It is the one safety error in this package that is about the CALLER rather
// than about the config: the same request from a wired console is fine, and the
// same request with the acknowledgement set is fine. That is deliberate — a
// router that cannot be reached after a change it applied correctly cannot
// explain itself, and the operator has no way back short of physical access.
//
// It also fires when the router cannot PROVE the caller is wired. See
// guardLockout.
var ErrLockoutRefused = errors.New("refusing to remove the access point you are connected " +
	"through, or cannot prove you are not: connect over Ethernet first, or resend " +
	"with acknowledge_lockout to accept losing WiFi access")

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

// wifiCallerIP reports whether clientIP may be connected over WiFi, reusing
// NetworkService.GetConnectionMethod rather than a second notion of "who is the
// caller": two classifiers would drift, and the one on the connection-method
// endpoint is what the operator's own UI already shows them.
//
// It is deliberately INCLUSIVE of `unknown`. The classifier cannot prove a
// caller is wired, and this guard's whole purpose is not to strand anyone, so
// "cannot tell" is treated as "might be on WiFi". See guardLockout.
func wifiCallerIP(deps classifyDeps, clientIP string) bool {
	switch classifyClientConnection(deps, clientIP).Method {
	case "ethernet":
		return false
	default:
		// wifi-client, wifi-ap and unknown all refuse. Only a PROVEN wired caller
		// passes; every other answer, including the ones the classifier could not
		// reach, is the direction that strands.
		return true
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
//
// IT FAILS CLOSED ON AN UNCLASSIFIED CALLER. wifiCallerIP refuses for every
// method except a proven `ethernet`, so a caller the classifier cannot place
// (an IP outside every interface prefix, a MAC with no neighbour entry, a bridge
// whose station dumps could not be read) is refused rather than allowed. That is
// the deliberate direction: a refusal costs the operator one acknowledgement
// they can read and undo, while allowing it strands them on a router that can no
// longer be reached to explain itself. The acknowledged path is unchanged, so
// the false positive is never more than a dialog. See ADR 0002 §5.2.
func (w *WifiService) guardLockout(req LockoutRequest, enabledAPRemains bool) error {
	if req.AcknowledgeLockout || req.ClientIP == "" {
		return nil
	}
	if enabledAPRemains {
		return nil
	}
	if !wifiCallerIP(w.classifyDeps(), req.ClientIP) {
		return nil
	}
	return ErrLockoutRefused
}

// classifyDeps builds the dependency set the shared classifier reads from, so
// the guard classifies its caller through exactly the code the
// GET /network/connection-method endpoint runs.
func (w *WifiService) classifyDeps() classifyDeps {
	return classifyDeps{ubus: w.ubus, cmd: w.cmd, arpFile: w.arpFile}
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
