package api

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/services"
	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// The interface every test request appears to arrive on under app.Test.
//
// app.Test's in-memory connection always reports 0.0.0.0 as the peer address
// (fiber's testConn), so a test cannot choose its own source IP the way a real
// phone does. What it CAN do is choose which interface that address is reachable
// through, by registering the `network.interface dump` answer the router really
// returns. That is what these two helpers do: same request, same 0.0.0.0 caller,
// a dump that classifies it as a WiFi client or as a wired one. The rule built
// on top of that classification is pinned on real addresses in
// internal/services/wifi_lockout_test.go.
// interfaceDump answers `ubus call network.interface dump` with one up L3 device
// carrying the REAL netifd shape — `ipv4-address` as a bare address plus a
// separate integer mask, which is what `ubus call network.interface dump`
// returns on the device. The previous fixture used an `ipv4-prefix` CIDR key the
// device never emits, so it described a payload no router produces.
//
// The device name is what decides the method: eth0 -> ethernet, br-lan -> a
// client on the LAN bridge. A br-lan caller is NOT resolved to `wifi-ap` here:
// that needs the caller's MAC and its association state, which these tests
// cannot supply, so it lands on `unknown` — which the guard also refuses. Every
// br-lan case below asserts a refusal, so the status and code are unaffected;
// the wired-vs-wireless discrimination itself is pinned in
// internal/services/client_classifier_test.go against the captured payload.
func interfaceDump(l3Device string) map[string]any {
	return map[string]any{
		"interface": []any{map[string]any{
			"interface":    l3Device,
			"up":           true,
			"device":       l3Device,
			"l3_device":    l3Device,
			"ipv4-address": []any{map[string]any{"address": "0.0.0.0", "mask": float64(0)}},
		}},
	}
}

func lockoutService(t *testing.T, reachableBy string) *services.WifiService {
	t.Helper()
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	ub.RegisterResponse("network.interface.dump", interfaceDump(reachableBy))
	return services.NewWifiServiceWithApplier(u, ub, &stubApplier{token: "lockout"})
}

func putFromIP(t *testing.T, app *fiber.App, path string, body any) (*http.Response, []byte) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPut, path, strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

// codeOf reads the machine-readable code out of an error body.
func codeOf(t *testing.T, body []byte) string {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("invalid JSON %s: %v", body, err)
	}
	code, _ := result["code"].(string)
	return code
}

func TestSetMode_LockoutRefusalIs409WithACode(t *testing.T) {
	app := fiber.New()
	u := uci.NewMockUCI()
	ub := ubus.NewMockUbus()
	ub.RegisterResponse("network.interface.dump", interfaceDump("br-lan"))
	svc := services.NewWifiServiceWithApplier(u, ub, &stubApplier{token: "lockout"})
	app.Put("/api/v1/wifi/mode", WifiSetModeHandler(svc))

	resp, body := putFromIP(t, app, "/api/v1/wifi/mode", map[string]any{"mode": "client"})

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", resp.StatusCode, body)
	}
	if got := codeOf(t, body); got != services.LockoutErrorCode {
		t.Errorf("code = %q, want %q", got, services.LockoutErrorCode)
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatal(err)
	}
	msg, _ := result["error"].(string)
	// The message has to name the remedy, or the operator does not know what to do.
	if !strings.Contains(msg, "Ethernet") {
		t.Errorf("error message does not name the remedy: %q", msg)
	}
	if v, _ := u.Get("wireless", "default_radio0", "disabled"); v == "1" {
		t.Error("refused mode switch disabled an access point")
	}
}

func TestSetMode_LockoutAcknowledgementIsAccepted(t *testing.T) {
	app := fiber.New()
	svc := lockoutService(t, "br-lan")
	app.Put("/api/v1/wifi/mode", WifiSetModeHandler(svc))

	resp, body := putFromIP(t, app, "/api/v1/wifi/mode",
		map[string]any{"mode": "client", "acknowledge_lockout": true})

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for an acknowledged switch, got %d: %s", resp.StatusCode, body)
	}
}

func TestSetMode_WiredCallerIsNotRefused(t *testing.T) {
	app := fiber.New()
	svc := lockoutService(t, "eth0")
	app.Put("/api/v1/wifi/mode", WifiSetModeHandler(svc))

	resp, body := putFromIP(t, app, "/api/v1/wifi/mode", map[string]any{"mode": "client"})

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for a wired operator, got %d: %s", resp.StatusCode, body)
	}
}

func TestSetAPConfig_DisablingTheLastAccessPointIsRefused(t *testing.T) {
	app := fiber.New()
	svc := lockoutService(t, "br-lan")
	app.Put("/api/v1/wifi/ap/:section", SetAPConfigHandler(svc))

	// The two default access points are the only ones, so taking the first down
	// still leaves the second: that one must go through.
	resp, body := putFromIP(t, app, "/api/v1/wifi/ap/default_radio0",
		map[string]any{"ssid": "OpenWrt-Travel", "enabled": false})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 while another access point stays up, got %d: %s",
			resp.StatusCode, body)
	}

	resp, body = putFromIP(t, app, "/api/v1/wifi/ap/default_radio1",
		map[string]any{"ssid": "OpenWrt-Travel-5G", "enabled": false})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for the last access point, got %d: %s", resp.StatusCode, body)
	}
	if got := codeOf(t, body); got != services.LockoutErrorCode {
		t.Errorf("code = %q, want %q", got, services.LockoutErrorCode)
	}
}

func TestSetAPConfig_SSIDChangeOverWiFiIsNotRefused(t *testing.T) {
	app := fiber.New()
	svc := lockoutService(t, "br-lan")
	app.Put("/api/v1/wifi/ap/:section", SetAPConfigHandler(svc))

	resp, body := putFromIP(t, app, "/api/v1/wifi/ap/default_radio0",
		map[string]any{"ssid": "Renamed", "encryption": "psk2", "key": "travelrouter"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for a rename over WiFi, got %d: %s", resp.StatusCode, body)
	}
}

func TestSetRadioEnabled_TurningTheRadiosOffIsRefused(t *testing.T) {
	app := fiber.New()
	svc := lockoutService(t, "br-lan")
	app.Put("/api/v1/wifi/radio", SetRadioEnabledHandler(svc))

	resp, body := putFromIP(t, app, "/api/v1/wifi/radio", map[string]any{"enabled": false})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 when the radios go off, got %d: %s", resp.StatusCode, body)
	}
	if got := codeOf(t, body); got != services.LockoutErrorCode {
		t.Errorf("code = %q, want %q", got, services.LockoutErrorCode)
	}
}

func TestSetRadioRole_TakingTheLastRadioOffIsRefused(t *testing.T) {
	app := fiber.New()
	svc := lockoutService(t, "br-lan")
	app.Put("/api/v1/wifi/radios/:name/role", SetRadioRoleHandler(svc))

	// radio0 hosts the only other access point, so switching radio1 off is safe.
	resp, body := putFromIP(t, app, "/api/v1/wifi/radios/radio1/role",
		map[string]any{"role": "none"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 while radio0's access point stays up, got %d: %s",
			resp.StatusCode, body)
	}

	resp, body = putFromIP(t, app, "/api/v1/wifi/radios/radio0/role",
		map[string]any{"role": "none"})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for the last radio, got %d: %s", resp.StatusCode, body)
	}
	if got := codeOf(t, body); got != services.LockoutErrorCode {
		t.Errorf("code = %q, want %q", got, services.LockoutErrorCode)
	}
}

func TestSetGuestWifi_DisablingTheOnlyAccessPointIsRefused(t *testing.T) {
	app := fiber.New()
	svc := lockoutService(t, "br-lan")
	app.Put("/api/v1/wifi/ap/:section", SetAPConfigHandler(svc))
	app.Put("/api/v1/wifi/guest", SetGuestWifiHandler(svc))

	// The two default access points go first (acknowledged): once they are down
	// guest WiFi is the only access point left, and taking it down is the lockout.
	for _, section := range []string{"default_radio0", "default_radio1"} {
		resp, body := putFromIP(t, app, "/api/v1/wifi/ap/"+section, map[string]any{
			"ssid": section, "enabled": false, "acknowledge_lockout": true})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("disabling %s: %d %s", section, resp.StatusCode, body)
		}
	}
	resp, body := putFromIP(t, app, "/api/v1/wifi/guest", map[string]any{
		"enabled": true, "ssid": "Guest", "encryption": "psk2", "key": "guestpass"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("enabling guest WiFi: %d %s", resp.StatusCode, body)
	}

	resp, body = putFromIP(t, app, "/api/v1/wifi/guest", map[string]any{"enabled": false})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for the last access point, got %d: %s", resp.StatusCode, body)
	}
	if got := codeOf(t, body); got != services.LockoutErrorCode {
		t.Errorf("code = %q, want %q", got, services.LockoutErrorCode)
	}
}

// ---------------------------------------------------------------------------
// The three mutators deliberately left OUT of the guard: POST /wifi/connect,
// POST /wifi/disconnect and POST /wifi/repeater/reconcile.
//
// "Not guarded" is not a property of an answer a test can read: it is the
// ABSENCE of a 409, and the previous version of these tests asserted that
// absence from a fixture built so the guard could not have refused anyway — no
// access point up at all — which made the assertion vacuous and left the real
// hole open, because a guard wired in the way guardLockoutExcluding's own doc
// comment prescribes went unnoticed.
//
// What IS readable is the property the guard is a proxy for, and it is the one
// the operator experiences: a caller the guard REFUSES (pinned below by
// TestGuardRefusal_PinsTheFixtureCallerAsNotProvenWired) still finds an access
// point that was up before their request up after it. So each fixture is the
// router's ordinary configuration with the uplink enabled — the everyday case —
// and each test asserts the IDENTITY of a surviving access point, not a count.
// A guard wired in with the documented default ("", "") returns nil on those
// fixtures because an access point genuinely remains, so these tests stay green
// under it, which is the honest statement: nothing was stranded. A narrowing
// exclude that WOULD strand the caller turns them red. The service-level
// version of the same property, including the layout split that decides it, is
// in internal/services/wifi_lockout_coverage_test.go.
// ---------------------------------------------------------------------------

// postFromIP is putFromIP for the POST routes.
func postFromIP(t *testing.T, app *fiber.App, path string, body any) (*http.Response, []byte) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, path, strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

// enabledAPs reads the enabled mode=ap wifi-ifaces straight out of the config
// the caller would be left with, rather than asking the guard whether one would
// survive: a test that reuses the guard's own definition of "an access point
// survives" proves only that the guard agrees with itself.
func enabledAPs(t *testing.T, u *uci.MockUCI) []string {
	t.Helper()
	sections, err := u.GetSections("wireless")
	if err != nil {
		t.Fatalf("reading wireless: %v", err)
	}
	var left []string
	for name, opts := range sections {
		if opts["mode"] != "ap" || opts["disabled"] == "1" {
			continue
		}
		radio, err := u.GetAll("wireless", opts["device"])
		if err != nil || radio["disabled"] == "1" {
			continue
		}
		left = append(left, name)
	}
	sort.Strings(left)
	return left
}

// assertSomeAPStillUp fails unless at least one access point that was up before
// the request is still up afterwards: the property "this cannot strand the
// caller", which is what the lockout guard exists to enforce. It is the right
// assertion for the mutators that MOVE an access point (Connect's split, the
// reconcile), where the set legitimately shrinks.
//
// Membership, not a count. A mutator that turned default_radio0 off and
// default_radio1 on leaves the same NUMBER of access points and has taken away
// the operator's way back in.
//
// The empty-fixture guard is deliberate: with nothing up beforehand the question
// has no answer and the call passes by construction, which is exactly how the
// previous version of this file carried a clause it never exercised.
func assertSomeAPStillUp(t *testing.T, before []string, u *uci.MockUCI) {
	t.Helper()
	after := assertAPFixtureHasSomethingUp(t, before, u)
	for _, name := range before {
		if hasName(after, name) {
			return
		}
	}
	t.Errorf("every access point that was up is now down: before %v, after %v", before, after)
}

// assertEveryAPStillUp is the stronger clause, for the mutator that moves
// nothing: Disconnect writes disabled=1 on one mode=sta section and does not so
// much as read a mode=ap one, so every access point that was up must still be
// up. "One survived" is not enough here — it is satisfied by exactly the swap
// that takes the operator's own access point away.
func assertEveryAPStillUp(t *testing.T, before []string, u *uci.MockUCI) {
	t.Helper()
	after := assertAPFixtureHasSomethingUp(t, before, u)
	up := map[string]bool{}
	for _, name := range after {
		up[name] = true
	}
	for _, name := range before {
		if !up[name] {
			t.Errorf("%s was up and is now down; access points before %v, after %v",
				name, before, after)
		}
	}
}

// assertAPFixtureHasSomethingUp reads the access points the caller was left
// with and refuses a fixture that had none up to begin with: with an empty
// `before` both callers above pass by construction, asserting nothing.
func assertAPFixtureHasSomethingUp(t *testing.T, before []string, u *uci.MockUCI) []string {
	t.Helper()
	if len(before) == 0 {
		t.Fatalf("fixture has no access point up, so nothing is being asserted")
	}
	return enabledAPs(t, u)
}

// assertNotLockoutRefusal fails when the body carries the lockout code, which
// is the one thing a wired-in guard would add to any of these three answers.
func assertNotLockoutRefusal(t *testing.T, body []byte) {
	t.Helper()
	if got := codeOf(t, body); got == services.LockoutErrorCode {
		t.Errorf("answered as a lockout refusal (%q): this endpoint is not guarded", got)
	}
}

// serviceForFixture builds the service for an unguarded-mutator test: the
// config handle comes back so the test can assert what the caller was left
// with, and the ubus answers are built FROM that config rather than invented —
// `network.wireless status` names the sta sections that really exist, so the
// lookup a mutator does (Disconnect) acts on a section the test set up instead
// of on the default mock's `wifinet2` profile, which is in no fixture.
func serviceForFixture(
	t *testing.T, reachableBy string, edit func(*testing.T, *uci.MockUCI),
) (*services.WifiService, *uci.MockUCI) {
	t.Helper()
	u := uci.NewMockUCI()
	if edit != nil {
		edit(t, u)
	}
	ub := ubus.NewMockUbus()
	ub.RegisterResponse("network.interface.dump", interfaceDump(reachableBy))
	ub.RegisterResponse("network.wireless.status", wirelessStatusFor(t, u))
	return services.NewWifiServiceWithApplier(u, ub, &stubApplier{token: "lockout"}), u
}

func wirelessStatusFor(t *testing.T, u *uci.MockUCI) map[string]any {
	t.Helper()
	sections, err := u.GetSections("wireless")
	if err != nil {
		t.Fatalf("reading wireless: %v", err)
	}
	radio := map[string]any{"interfaces": []any{}}
	for name, opts := range sections {
		if opts["mode"] != "sta" {
			continue
		}
		radio["interfaces"] = append(radio["interfaces"].([]any), map[string]any{
			"ifname":  "phy0-" + name,
			"section": name,
			"config":  map[string]any{"mode": "sta", "ssid": opts["ssid"]},
		})
	}
	return map[string]any{"radio0": radio}
}

// disableEveryAP leaves the router with NO enabled access point, which is the
// configuration the guard exists to catch. It is not used by the three mutator
// tests — it exists for the one test that PINS the classification their caller
// resolves to, so that "the guard would refuse this caller" is a checked fact
// rather than an assumption about the fixture (see
// TestGuardRefusal_PinsTheFixtureCallerAsNotProvenWired).
func disableEveryAP(t *testing.T, u *uci.MockUCI) {
	t.Helper()
	for _, section := range []string{"default_radio0", "default_radio1"} {
		if err := u.Set("wireless", section, "disabled", "1"); err != nil {
			t.Fatal(err)
		}
	}
}

// uplinkRadio reports the radio the enabled uplink STA sits on, "" when none is
// enabled — the observable evidence that a connect really re-homed the uplink
// rather than being quietly refused.
func uplinkRadio(t *testing.T, u *uci.MockUCI) string {
	t.Helper()
	sections, err := u.GetSections("wireless")
	if err != nil {
		t.Fatalf("reading wireless: %v", err)
	}
	for _, opts := range sections {
		if opts["mode"] == "sta" && opts["disabled"] != "1" {
			return opts["device"]
		}
	}
	return ""
}

// TestGuardRefusal_PinsTheFixtureCallerAsNotProvenWired pins the dependency the
// three tests below carry and used to leave implicit: their caller resolves to a
// method the guard REFUSES.
//
// A br-lan caller on the interface dump these tests register cannot be resolved
// to a MAC (the fixture supplies no neighbour entry), so the classifier answers
// `unknown` — which wifiCallerIP treats as unsafe. That is an artefact of the
// fixture, and an artefact nothing asserted: had the classifier been narrowed to
// refuse only a PROVEN wireless caller, these fixtures would have stopped being
// callers the guard refuses and the endpoint tests below would have gone quiet
// rather than red. This test is the tripwire — it fails the moment the
// classification changes.
func TestGuardRefusal_PinsTheFixtureCallerAsNotProvenWired(t *testing.T) {
	app := fiber.New()
	svc, _ := serviceForFixture(t, "br-lan", disableEveryAP)
	app.Put("/api/v1/wifi/ap/:section", SetAPConfigHandler(svc))

	resp, body := putFromIP(t, app, "/api/v1/wifi/ap/default_radio0",
		map[string]any{"ssid": "OpenWrt-Travel", "enabled": false})

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("fixture caller answered %d, want 409: it is no longer a caller the "+
			"guard refuses, so the endpoint tests below are testing something else",
			resp.StatusCode)
	}
	if got := codeOf(t, body); got != services.LockoutErrorCode {
		t.Errorf("code = %q, want %q", got, services.LockoutErrorCode)
	}
}

// The same refusal must not come from the CONFIG alone: with the identical
// fixture reached over a wired caller the request goes through, so the refusal
// above is the guard reading the caller and nothing else.
func TestGuardRefusal_IsNotAboutTheConfigAlone(t *testing.T) {
	app := fiber.New()
	svc, _ := serviceForFixture(t, "eth0", disableEveryAP)
	app.Put("/api/v1/wifi/ap/:section", SetAPConfigHandler(svc))

	resp, body := putFromIP(t, app, "/api/v1/wifi/ap/default_radio0",
		map[string]any{"ssid": "OpenWrt-Travel", "enabled": false})

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("wired caller answered %d, want 200: %s", resp.StatusCode, body)
	}
}

// Connect moves the uplink onto the 5 GHz radio, which carries default_radio1,
// so that access point goes. The other one has to be left up, or the caller who
// cannot be proven wired has no way back into the router.
func TestWifiConnect_KeepsAnAccessPointUpForAnUnclassifiedCaller(t *testing.T) {
	app := fiber.New()
	svc, u := serviceForFixture(t, "br-lan", nil)
	app.Post("/api/v1/wifi/connect", WifiConnectHandler(svc))
	before := enabledAPs(t, u)

	// The uplink moves to the 5 GHz radio, the split path's usual target.
	resp, body := postFromIP(t, app, "/api/v1/wifi/connect", map[string]any{
		"ssid": "Cafe-WiFi", "band": "5g", "encryption": "psk2", "password": "cafepass1",
	})

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("connect answered %d: %s", resp.StatusCode, body)
	}
	assertNotLockoutRefusal(t, body)
	if got := uplinkRadio(t, u); got != "radio1" {
		t.Errorf("uplink radio = %q, want radio1: the connect did not happen", got)
	}
	assertSomeAPStillUp(t, before, u)
}

// Disconnect drops the uplink and writes nothing else. Losing the uplink is not
// a lockout — the router still has its own WiFi to be reached on — but only if
// every access point really does come through the disconnect untouched.
func TestWifiDisconnect_KeepsAnAccessPointUpForAnUnclassifiedCaller(t *testing.T) {
	app := fiber.New()
	svc, u := serviceForFixture(t, "br-lan", nil)
	app.Post("/api/v1/wifi/disconnect", WifiDisconnectHandler(svc))
	before := enabledAPs(t, u)

	resp, body := postFromIP(t, app, "/api/v1/wifi/disconnect", map[string]any{})

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("disconnect answered %d: %s", resp.StatusCode, body)
	}
	assertNotLockoutRefusal(t, body)
	if dis, _ := u.Get("wireless", "sta0", "disabled"); dis != "1" {
		t.Errorf("sta0 disabled = %q, want \"1\": the disconnect did not happen", dis)
	}
	assertEveryAPStillUp(t, before, u)
}

// Reconcile re-derives the downlink layout: the uplink shares radio0, so
// default_radio0 goes and default_radio1 takes over as the operator's way back
// in. The access point MOVED. It must not simply have been removed and replaced
// by a number that happens to match, which is why this asserts membership of
// the before set and not the size of the after set.
func TestReconcileRepeater_KeepsAnAccessPointUpForAnUnclassifiedCaller(t *testing.T) {
	app := fiber.New()
	svc, u := serviceForFixture(t, "br-lan", nil)
	app.Post("/api/v1/wifi/reconcile", ReconcileRepeaterAPLayoutHandler(svc))
	before := enabledAPs(t, u)

	resp, body := postFromIP(t, app, "/api/v1/wifi/reconcile", map[string]any{})

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reconcile answered %d: %s", resp.StatusCode, body)
	}
	assertNotLockoutRefusal(t, body)
	// The reconcile re-homes the downlink onto the idle radio; if that stopped
	// happening the endpoint would be doing nothing at all and passing.
	if after := enabledAPs(t, u); !hasName(after, "default_radio1") {
		t.Errorf("enabled access points = %v, want the downlink re-homed to default_radio1",
			after)
	}
	assertSomeAPStillUp(t, before, u)
}

func hasName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

// A 409 that is NOT the lockout refusal must not carry the lockout code, or the
// frontend would raise the acknowledge dialog for an unrelated conflict.
func TestSetRadioRole_SameRadioConflictIsNotReportedAsLockout(t *testing.T) {
	app := fiber.New()
	u := uci.NewMockUCI()
	// An enabled uplink STA on radio0 and an access point still up on radio1, so
	// the request is refused for the radio layout, not for the lockout: an
	// access point would remain.
	if err := u.Set("wireless", "sta0", "network", "wwan"); err != nil {
		t.Fatal(err)
	}
	if err := u.Set("wireless", "sta0", "disabled", "0"); err != nil {
		t.Fatal(err)
	}
	ub := ubus.NewMockUbus()
	ub.RegisterResponse("network.interface.dump", interfaceDump("br-lan"))
	svc := services.NewWifiServiceWithApplier(u, ub, &stubApplier{token: "lockout"})
	app.Put("/api/v1/wifi/radios/:name/role", SetRadioRoleHandler(svc))

	resp, body := putFromIP(t, app, "/api/v1/wifi/radios/radio0/role",
		map[string]any{"role": "both"})

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for the same-radio conflict, got %d: %s", resp.StatusCode, body)
	}
	if got := codeOf(t, body); got == services.LockoutErrorCode {
		t.Error("the same-radio conflict must not be reported as a lockout")
	}
}
