// Package buttons lists the hardware buttons from the devicetree (device-facts §8) and maps
// a slide switch to one action from a fixed list. The switch's two positions arrive as
// procd hotplug events (`BUTTON=BTN_<n>`, `ACTION=pressed|released`); the Travo hotplug
// script hands them to the daemon's hook socket (ADR 0024), so the daemon's guards, locks
// and lockout rules apply. Push buttons (reset, WPS) keep their stock `/etc/rc.button`
// behaviour: they are listed, never configurable, and the script ignores them.
package buttons

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/blackdark-org/travo-greenfield/internal/device"
)

const (
	config = "travo-greenfield"
	// SectionPrefix marks the button sections in /etc/config/travo-greenfield.
	SectionPrefix = "button_"
	sectionType   = "button"

	evSW         = 5     // linux,input-type of a slide switch
	codeBTN0     = 0x100 // BTN_0; gpio-button-hotplug names BTN_0..BTN_9 as such
	codeBTN9     = 0x109
	codeRestart  = 0x198 // KEY_RESTART: the reset button, hotplug name "reset"
	codeWPS      = 0x211 // KEY_WPS_BUTTON
	codeRFKill   = 0xf7  // KEY_RFKILL
	positionOn   = "pressed"
	positionOff  = "released"
	actionPrefix = "button-"
)

// Action is what sliding the switch does; the pressed position turns it on.
type Action string

const (
	None      Action = "none"
	VPN       Action = "vpn"        // WireGuard tunnel on / off
	GuestWiFi Action = "guest_wifi" // guest access point on / off
)

var (
	// ErrInvalid: an unknown action.
	ErrInvalid = errors.New("invalid button action")
	// ErrNotFound: no button with that devicetree name.
	ErrNotFound = errors.New("button not found")
	// ErrNotConfigurable: a push button (reset, WPS) keeps its stock behaviour.
	ErrNotConfigurable = errors.New("button keeps its stock behaviour")
)

// Valid reports whether a is one of the fixed actions.
func (a Action) Valid() bool { return a == None || a == VPN || a == GuestWiFi }

// Button is one hardware button as the API shows it.
type Button struct {
	Name         string `json:"name" doc:"Devicetree key node"`
	Label        string `json:"label" doc:"Devicetree label"`
	Hotplug      string `json:"hotplug" doc:"BUTTON value of its hotplug event; empty when unknown"`
	Configurable bool   `json:"configurable" doc:"A slide switch Travo may map; push buttons keep their stock behaviour"`
	Action       Action `json:"action" enum:"none,vpn,guest_wifi"`
	Last         *Run   `json:"last,omitempty" doc:"Last slide since the daemon started"`
}

// Status is what the API reads.
type Status struct {
	Buttons []Button `json:"buttons" nullable:"false"`
}

// HotplugName is gpio-button-hotplug's BUTTON name for a key code; "" when unknown.
func HotplugName(code uint32) string {
	switch {
	case code >= codeBTN0 && code <= codeBTN9:
		return fmt.Sprintf("BTN_%d", code-codeBTN0)
	case code == codeRestart:
		return "reset"
	case code == codeWPS:
		return "wps"
	case code == codeRFKill:
		return "rfkill"
	}
	return ""
}

// Configurable: only slide switches on BTN_0..BTN_9, the codes the hotplug script passes on.
func Configurable(b device.Button) bool {
	return b.InputType == evSW && b.Code >= codeBTN0 && b.Code <= codeBTN9
}

// ActionName is the hook action the hotplug script calls for one switch position:
// BTN_0 pressed is `button-btn-0-pressed`.
func ActionName(hotplug, position string) string {
	return actionPrefix + strings.ToLower(strings.ReplaceAll(hotplug, "_", "-")) + "-" + position
}

// SectionName is the UCI section of a devicetree node; characters UCI rejects become `_`.
func SectionName(node string) string {
	b := []byte(node)
	for i, c := range b {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_') {
			b[i] = '_'
		}
	}
	return SectionPrefix + string(b)
}

// Configured maps each node to its stored action. An unknown stored action reads as none.
func Configured(secs []device.Section) map[string]Action {
	out := map[string]Action{}
	for _, s := range secs {
		if s.Type != sectionType || !strings.HasPrefix(s.Name, SectionPrefix) {
			continue
		}
		if a := Action(s.Options["action"]); a.Valid() {
			out[s.Options["node"]] = a
		}
	}
	return out
}

// View joins the devicetree buttons with their stored actions and last runs.
func View(keys []device.Button, actions map[string]Action, runs map[string]Run) Status {
	st := Status{Buttons: make([]Button, 0, len(keys))}
	for _, k := range keys {
		b := Button{Name: k.Node, Label: k.Label, Hotplug: HotplugName(k.Code), Configurable: Configurable(k), Action: None}
		if b.Configurable {
			if a, ok := actions[k.Node]; ok {
				b.Action = a
			}
			if r, ok := runs[k.Node]; ok {
				b.Last = &r
			}
		}
		st.Buttons = append(st.Buttons, b)
	}
	return st
}

// find returns the configurable button named node.
func find(keys []device.Button, node string) (device.Button, error) {
	at := slices.IndexFunc(keys, func(k device.Button) bool { return k.Node == node })
	if at < 0 {
		return device.Button{}, ErrNotFound
	}
	if !Configurable(keys[at]) {
		return device.Button{}, ErrNotConfigurable
	}
	return keys[at], nil
}
