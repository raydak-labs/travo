package api

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/services"
)

// The served /api/openapi.json is the contract automation and tests depend on,
// so it must not silently drift from the routes the API actually registers.
//
// This test compares every registered operation under /api/v1 against the
// operations declared in the spec. It fails on BOTH directions:
//   - a registered route missing from the spec (undocumented endpoint), and
//   - a spec entry with no matching route (a promise the server does not keep).
type openAPIOperation struct {
	method string
	path   string
}

func (o openAPIOperation) String() string { return strings.ToUpper(o.method) + " " + o.path }

// openAPIPath normalises a registered route path to the OpenAPI template form.
// Fiber registers path parameters as ":section" while OpenAPI spells them
// "{section}"; without this the two sides can never be compared.
func openAPIPath(path string) string {
	var b strings.Builder
	for i := 0; i < len(path); i++ {
		if path[i] != ':' {
			b.WriteByte(path[i])
			continue
		}
		j := i + 1
		for j < len(path) && (path[j] == '_' || path[j] >= 'a' && path[j] <= 'z' || path[j] >= 'A' && path[j] <= 'Z' || path[j] >= '0' && path[j] <= '9') {
			j++
		}
		if j == i+1 {
			b.WriteByte(path[i])
			continue
		}
		b.WriteByte('{')
		b.WriteString(path[i+1 : j])
		b.WriteByte('}')
		i = j - 1
	}
	return b.String()
}

func setupSpecApp(t *testing.T) *fiber.App {
	t.Helper()
	app, deps := setupTestApp(t)
	// The shared test app already called SetupRoutes; re-registering is not
	// needed, we only need the app object to introspect and to serve the spec.
	_ = deps
	return app
}

// registeredOperations returns the operations registered under /api/v1,
// excluding Fiber's automatic HEAD routes and the unauthenticated meta routes
// that are not part of the versioned contract.
func registeredOperations(t *testing.T, app *fiber.App) map[openAPIOperation]bool {
	t.Helper()
	out := make(map[openAPIOperation]bool)
	for _, r := range app.GetRoutes() {
		if !strings.HasPrefix(r.Path, "/api/v1/") {
			continue
		}
		method := strings.ToUpper(r.Method)
		if method == "HEAD" {
			continue // auto-generated companion of every GET
		}
		if r.Path == "/api/v1/ws" {
			continue // WebSocket upgrade, documented outside the REST contract
		}
		out[openAPIOperation{method: method, path: openAPIPath(strings.TrimPrefix(r.Path, "/api/v1"))}] = true
	}
	return out
}

// specOperations returns the operations declared in the OpenAPI document.
func specOperations(t *testing.T, app *fiber.App) map[openAPIOperation]bool {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "/api/openapi.json", nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/openapi.json: got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(body, &spec); err != nil {
		t.Fatalf("spec is not valid JSON: %v", err)
	}
	ops := make(map[openAPIOperation]bool)
	for path, methods := range spec.Paths {
		for method := range methods {
			ops[openAPIOperation{method: strings.ToUpper(method), path: path}] = true
		}
	}
	return ops
}

func TestOpenAPISpecHasNoRouteDrift(t *testing.T) {
	app := setupSpecApp(t)
	registered := registeredOperations(t, app)
	spec := specOperations(t, app)

	if len(registered) == 0 {
		t.Fatal("no routes introspected — the comparison would be vacuous")
	}

	var undocumented, unimplemented []string
	for op := range registered {
		if !spec[op] {
			undocumented = append(undocumented, op.String())
		}
	}
	for op := range spec {
		if !registered[op] {
			unimplemented = append(unimplemented, op.String())
		}
	}
	sort.Strings(undocumented)
	sort.Strings(unimplemented)

	if len(undocumented) > 0 {
		t.Errorf("%d registered operations are missing from /api/openapi.json:\n  %s",
			len(undocumented), strings.Join(undocumented, "\n  "))
	}
	if len(unimplemented) > 0 {
		t.Errorf("%d spec operations have no matching route:\n  %s",
			len(unimplemented), strings.Join(unimplemented, "\n  "))
	}
}

// Every spec operation must be described, not just present: an operation with
// no summary is how the drift becomes invisible in review.
func TestOpenAPIOperationsAreDescribed(t *testing.T) {
	app := setupSpecApp(t)
	req, _ := http.NewRequest(http.MethodGet, "/api/openapi.json", nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	var spec struct {
		Paths map[string]map[string]struct {
			Summary   string `json:"summary"`
			Responses map[string]struct {
				Description string `json:"description"`
			} `json:"responses"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(body, &spec); err != nil {
		t.Fatalf("spec is not valid JSON: %v", err)
	}
	for path, methods := range spec.Paths {
		for method, op := range methods {
			where := strings.ToUpper(method) + " " + path
			if strings.TrimSpace(op.Summary) == "" {
				t.Errorf("%s has no summary", where)
			}
			if len(op.Responses) == 0 {
				t.Errorf("%s declares no responses", where)
				continue
			}
			if _, ok := op.Responses["200"]; !ok {
				t.Errorf("%s has no 200 response", where)
			}
		}
	}
}

// specResponseKeys returns the property names the spec declares in the 200
// response example of an operation, e.g. ["status", "token"].
//
// path is a real request path ("/api/v1/system/hostname"); spec paths are
// declared relative to the "/api/v1" server entry, so the prefix is stripped.
func specResponseKeys(t *testing.T, spec map[string]any, path, method string) []string {
	t.Helper()
	paths, _ := spec["paths"].(map[string]any)
	path = strings.TrimPrefix(path, "/api/v1")
	ops, _ := paths[path].(map[string]any)
	op, _ := ops[strings.ToLower(method)].(map[string]any)
	responses, _ := op["responses"].(map[string]any)
	ok, _ := responses["200"].(map[string]any)
	content, _ := ok["content"].(map[string]any)
	// The spec declares a single content type per operation; take whichever
	// is there rather than hardcoding "application/json".
	for _, v := range content {
		schema, _ := v.(map[string]any)["schema"].(map[string]any)
		example, _ := schema["example"].(map[string]any)
		keys := make([]string, 0, len(example))
		for k := range example {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys
	}
	return nil
}

// The drift test above compares (method, path) only, so it structurally cannot
// see a handler that changes the SHAPE of its response while keeping its route.
// That is not hypothetical: this branch changed four responses in place — most
// importantly PUT /auth/password, which now returns a replacement token because
// the change revokes every session. A client written against the documented
// contract keeps using the revoked token and is silently logged out.
//
// This pins the 200-response keys of those endpoints in BOTH directions: the
// spec must declare them, the handler must return them, and neither may carry a
// key the other lacks.
func TestOpenAPIResponseShapesMatchHandlers(t *testing.T) {
	spec := openAPISpec

	tests := []struct {
		name      string
		method    string
		path      string
		body      string
		authToken string
		wantKeys  []string
	}{
		{
			name:   "ChangePassword returns the replacement token",
			method: "PUT", path: "/api/v1/auth/password",
			body:     `{"current_password":"admin","new_password":"newpassword123"}`,
			wantKeys: []string{"status", "token", "expires_at", "expires_in", "revoked_sessions"},
		},
		{
			name:   "SetHostname reports reboot_required",
			method: "PUT", path: "/api/v1/system/hostname",
			body:     `{"hostname":"travo-test"}`,
			wantKeys: []string{"status", "reboot_required"},
		},
		{
			name:   "SetTimezone reports reboot_required",
			method: "PUT", path: "/api/v1/system/timezone",
			body:     `{"zonename":"Europe/Berlin","timezone":"CET"}`,
			wantKeys: []string{"status", "reboot_required"},
		},
		{
			name:   "SetNTP reports reboot_required",
			method: "PUT", path: "/api/v1/system/ntp",
			body:     `{"servers":["0.openwrt.pool.ntp.org"]}`,
			wantKeys: []string{"status", "reboot_required"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// A fresh app per subtest: PUT /auth/password changes the password,
			// so sharing one AuthService would make every later subtest fail to
			// log in.
			app, deps := setupTestApp(t)
			token, _, err := deps.Auth.Login("admin")
			if err != nil {
				t.Fatalf("login: %v", err)
			}

			declared := specResponseKeys(t, spec, tc.path, tc.method)
			for _, want := range tc.wantKeys {
				if !slices.Contains(declared, want) {
					t.Errorf("OpenAPI %s %s does not declare %q (declares %v)", tc.method, tc.path, want, declared)
				}
			}

			req, _ := http.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			respBody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("handler returned %d: %s", resp.StatusCode, respBody)
			}
			var got map[string]any
			if err := json.Unmarshal(respBody, &got); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			for _, want := range tc.wantKeys {
				if _, ok := got[want]; !ok {
					t.Errorf("handler is missing %q, which the spec documents; got %s", want, respBody)
				}
			}
			for key := range got {
				if !slices.Contains(tc.wantKeys, key) {
					t.Errorf("handler returns %q but the spec does not declare it", key)
				}
			}
		})
	}
}

// specRequestKeys returns the property names the spec declares in an
// operation's requestBody example, e.g. ["ip", "name"].
func specRequestKeys(t *testing.T, spec map[string]any, path, method string) []string {
	t.Helper()
	paths, _ := spec["paths"].(map[string]any)
	path = strings.TrimPrefix(path, "/api/v1")
	ops, _ := paths[path].(map[string]any)
	op, _ := ops[strings.ToLower(method)].(map[string]any)
	body, _ := op["requestBody"].(map[string]any)
	content, _ := body["content"].(map[string]any)
	for _, v := range content {
		schema, _ := v.(map[string]any)["schema"].(map[string]any)
		example, _ := schema["example"].(map[string]any)
		keys := make([]string, 0, len(example))
		for k := range example {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys
	}
	return nil
}

// The response-shape test above only covers operations whose 200 shape this
// branch happened to change, and it compares the SPEC to a HANDLER-WRITTEN
// expectation — never the spec to the handler for request bodies. So a spec
// that names a request field the handler does not accept is invisible: the
// documented request is simply rejected.
//
// That is not hypothetical. /api/openapi.json advertised "hostname" for
// POST /network/dns/entries and POST /network/dhcp/reservations, while both
// handlers bind models.DNSEntry / models.DHCPReservation, whose field is
// "name". A client generated from the documented contract got a 400 on both,
// with no failing test anywhere.
//
// This sends each documented example, filled in with values the handler
// accepts, and requires it to be accepted. The keys come from the spec, so
// renaming a field in either place fails here.
func TestOpenAPIRequestBodiesAreAccepted(t *testing.T) {
	spec := openAPISpec

	// A plausible value per field, so the handler's own validation is not what
	// rejects the request. Only the FIELD NAMES come from the spec.
	values := map[string]any{
		"name": "openapi-drift-probe", "ip": "10.9.9.9", "mac": "aa:bb:cc:dd:ee:ff",
		"hostname": "openapi-drift-probe", "servers": []string{"1.1.1.1"},
	}

	// Endpoints whose handler can be driven end to end against the shared test
	// app. Deliberately excludes the live-state operations (reboot, factory
	// reset, firmware, wifi connect/disconnect, interface state) and anything
	// needing hardware or an installed package.
	//
	// This is intentionally short. It is the only assertion that needs a running
	// app, and the static TestOpenAPIRequestFieldNamesMatchModels below covers
	// the same class of drift for every documented request body without one.
	endpoints := []struct{ method, path string }{
		{"POST", "/api/v1/network/dns/entries"},
		{"POST", "/api/v1/network/dhcp/reservations"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			// A fresh app per subtest: these write persisted config, and sharing
			// one app would let one subtest's state satisfy another's assertions.
			app, deps := setupTestApp(t)
			token, _, err := deps.Auth.Login("admin")
			if err != nil {
				t.Fatalf("login: %v", err)
			}

			keys := specRequestKeys(t, spec, ep.path, ep.method)
			if len(keys) == 0 {
				t.Fatalf("the spec declares no requestBody example for %s %s, so this test would be vacuous",
					ep.method, ep.path)
			}
			payload := map[string]any{}
			for _, k := range keys {
				v, ok := values[k]
				if !ok {
					t.Fatalf("no probe value for documented field %q; add one so this test keeps testing the contract", k)
				}
				payload[k] = v
			}
			raw, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("marshal payload: %v", err)
			}

			req, _ := http.NewRequest(ep.method, ep.path, strings.NewReader(string(raw)))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode >= 400 {
				t.Errorf("the request the spec documents (%s) was rejected with %d: %s\n"+
					"a client generated from /api/openapi.json cannot use this endpoint",
					raw, resp.StatusCode, body)
			}
		})
	}
}

// The drift tests above compare (method, path) and the 200-response shape. A spec
// that names a request field the handler does not accept is a third, separate
// class — and it is the one that breaks generated clients, because the request
// they build is rejected with a 400 that says nothing about the field name.
//
// Two real instances, both found on the device by this test class:
//   - POST /network/dns/entries and POST /network/dhcp/reservations documented
//     "hostname" while the handlers bind models.DNSEntry / models.DHCPReservation,
//     whose field is "name" → 400 "name is required".
//   - PUT /network/dhcp documented "leasetime" while models.DHCPConfig tags it
//     "lease_time" → 400 "lease_time is required".
//
// This compares the spec's declared request keys against the JSON tags of the
// struct each handler binds, by reflection. It needs no running app, so it covers
// every documented config endpoint rather than the two that can be driven here.
func TestOpenAPIRequestFieldNamesMatchModels(t *testing.T) {
	spec := openAPISpec

	// SyncTimeHandler binds this shape inline; it is reproduced here so renaming
	// the field on either side fails.
	type timeSyncRequest struct {
		ClientTimeMs int64 `json:"client_time_ms"`
	}

	tests := []struct {
		method string
		path   string
		model  any // a pointer to the struct the handler binds
	}{
		{"POST", "/api/v1/network/dns/entries", &models.DNSEntry{}},
		{"POST", "/api/v1/network/dhcp/reservations", &models.DHCPReservation{}},
		{"PUT", "/api/v1/network/dhcp", &models.DHCPConfig{}},
		{"PUT", "/api/v1/network/dns", &models.DNSConfig{}},
		{"PUT", "/api/v1/network/doh", &models.DoHConfig{}},
		{"PUT", "/api/v1/network/ddns", &models.DDNSConfig{}},
		{"PUT", "/api/v1/network/failover", &models.FailoverConfig{}},
		{"PUT", "/api/v1/wifi/schedule", &models.WiFiSchedule{}},
		{"PUT", "/api/v1/wifi/ap/{section}", &models.APConfigUpdate{}},
		{"PUT", "/api/v1/system/timezone", &models.TimezoneConfig{}},
		{"PUT", "/api/v1/system/ntp", &models.NTPConfig{}},
		{"PUT", "/api/v1/system/alert-thresholds", &models.AlertThresholds{}},
		// The pre-login clock-recovery path documented "timestamp" while the
		// handler binds client_time_ms and answers 400 without it, so a client
		// generated from the spec could not use the endpoint at all.
		{"POST", "/api/v1/system/time-sync", &timeSyncRequest{}},
		// PUT /system/leds documented "enabled" while the handler binds
		// models.SetLEDRequest.StealthMode: the documented request was accepted,
		// ignored, and answered 200 after doing the OPPOSITE of what was asked.
		{"PUT", "/api/v1/system/leds", &models.SetLEDRequest{}},
		{"PUT", "/api/v1/sqm/config", &models.SQMConfig{}},
		{"PUT", "/api/v1/vpn/split-tunnel", &models.SplitTunnelConfig{}},
		{"PUT", "/api/v1/vpn/wireguard", &models.WireguardConfig{}},
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			declared := specRequestKeys(t, spec, tc.path, tc.method)
			if len(declared) == 0 {
				t.Fatalf("the spec declares no requestBody example for %s %s, so this test would be vacuous",
					tc.method, tc.path)
			}

			accepted := jsonFieldNames(reflect.TypeOf(tc.model))

			// Direction 1: the spec must not promise a field the handler drops.
			for _, d := range declared {
				if !slices.Contains(accepted, d) {
					t.Errorf("the spec documents request field %q but %s has no such field (accepts %v): "+
						"a generated client sends it and gets 400", d, tc.model, accepted)
				}
			}
			// Direction 2: the handler must not require a field the spec omits.
			// Not every model field is required, so this lists them for review
			// rather than failing outright — an empty documented example is the
			// usual cause and is itself worth seeing.
			var undeclared []string
			for _, a := range accepted {
				if !slices.Contains(declared, a) {
					undeclared = append(undeclared, a)
				}
			}
			if len(undeclared) > 0 {
				t.Logf("note: %s accepts %v, which the spec does not document", tc.model, undeclared)
			}
		})
	}
}

// jsonFieldNames returns the JSON names a struct will decode from, following
// embedded structs the way encoding/json does.
func jsonFieldNames(t reflect.Type) []string {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	var out []string
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		if name == "" {
			if f.Anonymous && f.Type.Kind() == reflect.Struct {
				out = append(out, jsonFieldNames(f.Type)...)
				continue
			}
			name = f.Name
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Every wireless mutator answers ONE envelope, built by wifiMutationResponse
// (wifi_handlers.go): {"status":"ok","apply":{pending, token,
// rollback_timeout_seconds}}, sometimes with one endpoint-specific key added.
//
// The spec documented two shapes that no handler produces —
// {"token","confirm_within_seconds"} for POST /wifi/connect and PUT
// /wifi/ap/{section}, and {"ok":…} for the rest — so a client generated from
// /api/openapi.json read fields that are never sent and missed the fields that
// are. This compares the declared 200 keys against the envelope the handler
// actually builds, so renaming a key in either place fails here.
//
// Handlers that answer the bare apply result instead of the envelope
// (PUT /wifi/radios/{name}/role returns *services.WirelessApplyResult) are
// deliberately not listed: the spec documents the envelope they are supposed to
// answer.
func TestOpenAPIWifiMutationResponsesMatchEnvelope(t *testing.T) {
	spec := openAPISpec

	envelope := wifiMutationResponse(&services.WirelessApplyResult{
		Token:                  "apply-token",
		RollbackTimeoutSeconds: 90,
	})
	envelopeKeys := make([]string, 0, len(envelope))
	for k := range envelope {
		envelopeKeys = append(envelopeKeys, k)
	}
	sort.Strings(envelopeKeys)

	apply, ok := envelope["apply"].(fiber.Map)
	if !ok {
		t.Fatalf("the apply envelope is not a fiber.Map: %T", envelope["apply"])
	}
	applyKeys := make([]string, 0, len(apply))
	for k := range apply {
		applyKeys = append(applyKeys, k)
	}
	sort.Strings(applyKeys)

	tests := []struct {
		method    string
		path      string
		extraKeys []string // keys the handler adds on top of the envelope
	}{
		{"POST", "/api/v1/wifi/connect", nil},
		{"POST", "/api/v1/wifi/disconnect", nil},
		{"PUT", "/api/v1/wifi/mode", nil},
		{"PUT", "/api/v1/wifi/radio", nil},
		{"PUT", "/api/v1/wifi/ap/{section}", nil},
		{"PUT", "/api/v1/wifi/repeater-options", []string{"allow_ap_on_sta_radio"}},
		{"POST", "/api/v1/wifi/repeater/reconcile", nil},
		{"PUT", "/api/v1/wifi/mac", nil},
		{"POST", "/api/v1/wifi/mac/randomize", []string{"mac"}},
		{"PUT", "/api/v1/wifi/saved/priority", nil},
		{"PUT", "/api/v1/wifi/guest", nil},
	}

	if len(applyKeys) == 0 {
		t.Fatal("the apply envelope carries no keys, so this test would be vacuous")
	}

	for _, tc := range tests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			declared := specResponseKeys(t, spec, tc.path, tc.method)
			if len(declared) == 0 {
				t.Fatalf("the spec declares no 200 example for %s %s, so this test would be vacuous",
					tc.method, tc.path)
			}
			want := append(append([]string{}, envelopeKeys...), tc.extraKeys...)
			sort.Strings(want)

			if !slices.Equal(declared, want) {
				t.Errorf("OpenAPI %s %s declares %v, but the handler answers %v",
					tc.method, tc.path, declared, want)
			}
			// The apply sub-object must be documented too: it carries the token
			// the browser has to confirm with.
			declaredApply, _ := specApplyKeys(t, spec, tc.path, tc.method)
			if !slices.Equal(declaredApply, applyKeys) {
				t.Errorf("OpenAPI %s %s documents apply as %v, the handler sends %v",
					tc.method, tc.path, declaredApply, applyKeys)
			}
		})
	}
}

// specApplyKeys returns the property names declared inside the 200 response's
// "apply" object.
func specApplyKeys(t *testing.T, spec map[string]any, path, method string) ([]string, bool) {
	t.Helper()
	paths, _ := spec["paths"].(map[string]any)
	ops, _ := paths[strings.TrimPrefix(path, "/api/v1")].(map[string]any)
	op, _ := ops[strings.ToLower(method)].(map[string]any)
	responses, _ := op["responses"].(map[string]any)
	ok200, _ := responses["200"].(map[string]any)
	content, _ := ok200["content"].(map[string]any)
	for _, v := range content {
		schema, _ := v.(map[string]any)["schema"].(map[string]any)
		example, _ := schema["example"].(map[string]any)
		apply, ok := example["apply"].(map[string]any)
		if !ok {
			return nil, false
		}
		keys := make([]string, 0, len(apply))
		for k := range apply {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys, true
	}
	return nil, false
}

// ---------------------------------------------------------------------------
// Strict-body gate
// ---------------------------------------------------------------------------

// BindStrictBodyConfig rejects an unknown field, which turns "the body did not
// match the shape I expected" into a 400 that names the field. The permissive
// c.Bind().Body() drops it silently, and a handler that then persists the bound
// struct overwrites the user's settings with zeros while answering 200. ADR
// 0002 §2 documents one field that is deliberately NOT persisted
// (allow_ap_on_sta_radio) — exactly the kind of field the permissive binder
// swallows without a word.
//
// The ratchet below is derived from the SPEC, not from a remembered list of
// endpoints: every registered PUT whose documented requestBody is a JSON object
// with more than one property replaces a whole configuration object, so its
// handler must bind strictly. A single-property body cannot zero a
// multi-setting configuration, so it is out of scope — as are POSTs, which
// create a resource instead of replacing one.
//
// The handlers are classified by parsing this package's own source, so the gate
// needs no running app, never touches live state, and cannot be satisfied by a
// route that silently stopped binding at all.

// knownPermissiveWholeConfigHandlers are whole-config PUTs outside this lane
// that still bind with the permissive binder. They are listed rather than
// hidden so the gap stays visible; each one fixed elsewhere lets its entry be
// deleted here.
var knownPermissiveWholeConfigHandlers = map[string]string{
	// auth_handlers.go: ChangePasswordHandler
	"PUT /auth/password": "auth_handlers.go (password change, not a config write)",
	// adguard_handlers.go: SetAdGuardPasswordHandler
	"PUT /adguard/password": "adguard_handlers.go (AdGuard UI password, not a config write)",
	// wifi_handlers.go: SetAPConfigHandler
	"PUT /wifi/ap/{section}": "wifi_handlers.go — AP section update binds permissively",
}

func TestWholeConfigPutHandlersBindStrictly(t *testing.T) {
	app := setupSpecApp(t)
	registered := registeredOperations(t, app)
	spec := openAPISpec

	funcs, routeHandlers := parseAPIPackage(t)

	classified := 0
	for op := range registered {
		if op.method != http.MethodPut {
			continue
		}
		keys, ok := specJSONBodyKeys(spec, op)
		if !ok || len(keys) < 2 {
			continue // not a documented multi-field JSON object: not a whole-config write
		}
		classified++

		where := op.String()
		handler, ok := routeHandlers[op]
		if !ok {
			t.Errorf("%s is a whole-config write but its handler could not be resolved in the source; "+
				"update parseAPIPackage so this gate keeps covering every route", where)
			continue
		}
		fn, ok := funcs[handler]
		if !ok {
			t.Errorf("%s: handler %s was not found in this package", where, handler)
			continue
		}
		switch handlerBinder(fn) {
		case binderStrict:
			// good
		case binderPermissive:
			if reason, known := knownPermissiveWholeConfigHandlers[where]; known {
				t.Logf("known gap: %s binds permissively (%s)", where, reason)
				continue
			}
			t.Errorf("%s replaces a whole configuration object (%v) but %s binds with c.Bind().Body(), "+
				"so a body naming the wrong field is accepted, ignored and answered 200; "+
				"use BindStrictBodyConfig",
				where, keys, handler)
		default:
			t.Errorf("%s: %s neither calls BindStrictBodyConfig nor binds a body; "+
				"this gate can no longer tell whether it is strict", where, handler)
		}
	}

	if classified == 0 {
		t.Fatal("no whole-config PUT was classified — the gate would be vacuous")
	}
	if len(knownPermissiveWholeConfigHandlers) > 0 {
		for op := range registered {
			if _, ok := knownPermissiveWholeConfigHandlers[op.String()]; ok {
				if binder := handlerBinder(funcs[routeHandlers[op]]); binder != binderPermissive {
					t.Logf("stale ratchet entry: %s is no longer permissive, "+
						"delete it from knownPermissiveWholeConfigHandlers", op)
				}
			}
		}
	}
}

type bodyBinder int

const (
	binderUnknown bodyBinder = iota
	binderStrict
	binderPermissive
)

// handlerBinder reports which binder a handler function uses. decodeStrictJSON
// counts as strict: SetBandSwitchingHandler calls it directly for both the
// wrapped and the bare shape.
func handlerBinder(fn *ast.FuncDecl) bodyBinder {
	if fn == nil || fn.Body == nil {
		return binderUnknown
	}
	found := binderUnknown
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			if fun.Name == "BindStrictBodyConfig" || fun.Name == "decodeStrictJSON" {
				found = binderStrict
			}
		case *ast.SelectorExpr:
			// c.Bind().Body(&req) is a SelectorExpr on a CallExpr.
			if fun.Sel.Name != "Body" {
				return true
			}
			if inner, ok := fun.X.(*ast.CallExpr); ok {
				if sel, ok := inner.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Bind" {
					found = binderPermissive
				}
			}
		}
		return true
	})
	return found
}

// parseAPIPackage parses this package's non-test sources and returns the
// handler functions plus the route → handler-constructor mapping taken from the
// route registrations.
func parseAPIPackage(t *testing.T) (map[string]*ast.FuncDecl, map[openAPIOperation]string) {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read api package dir: %v", err)
	}
	funcs := make(map[string]*ast.FuncDecl)
	routeHandlers := make(map[openAPIOperation]string)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		{
			file, err := parser.ParseFile(fset, name, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil {
					continue
				}
				funcs[fn.Name.Name] = fn
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					switch strings.ToUpper(sel.Sel.Name) {
					case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete:
					default:
						return true
					}
					recv, ok := sel.X.(*ast.Ident)
					if !ok || (recv.Name != "app" && recv.Name != "v1") {
						return true
					}
					if len(call.Args) < 2 {
						return true
					}
					lit, ok := call.Args[0].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						return true
					}
					ctor, ok := call.Args[1].(*ast.CallExpr)
					if !ok {
						return true
					}
					ident, ok := ctor.Fun.(*ast.Ident)
					if !ok {
						return true // an inline func literal: not a named handler
					}
					path := openAPIPath(strings.TrimPrefix(strings.Trim(lit.Value, `"`), "/api/v1"))
					routeHandlers[openAPIOperation{
						method: strings.ToUpper(sel.Sel.Name),
						path:   path,
					}] = ident.Name
					return true
				})
			}
		}
	}
	if len(routeHandlers) == 0 {
		t.Fatal("no routes were parsed from the source; the strict-binding gate would be vacuous")
	}
	return funcs, routeHandlers
}

// specJSONBodyKeys returns the property names the spec documents in an
// operation's application/json requestBody example.
func specJSONBodyKeys(spec map[string]any, op openAPIOperation) ([]string, bool) {
	paths, _ := spec["paths"].(map[string]any)
	ops, _ := paths[op.path].(map[string]any)
	entry, _ := ops[strings.ToLower(op.method)].(map[string]any)
	body, _ := entry["requestBody"].(map[string]any)
	content, _ := body["content"].(map[string]any)
	media, ok := content["application/json"].(map[string]any)
	if !ok {
		return nil, false
	}
	schema, _ := media["schema"].(map[string]any)
	example, _ := schema["example"].(map[string]any)
	if len(example) == 0 {
		return nil, false
	}
	keys := make([]string, 0, len(example))
	for k := range example {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, true
}

// The strict-binding gate above is only worth anything if the classifier can
// tell the two binders apart, so pin it on known sources.
func TestHandlerBinderClassifiesBinders(t *testing.T) {
	const src = `package api

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
)

func strictHandler(c fiber.Ctx) error {
	var cfg struct{ Enabled bool ` + "`json:\"enabled\"`" + ` }
	if err := BindStrictBodyConfig(c, &cfg); err != nil {
		return err
	}
	return nil
}

func strictDecodeHandler(c fiber.Ctx) error {
	var cfg struct{ Enabled bool }
	return decodeStrictJSON(c.Body(), &cfg, "")
}

func permissiveHandler(c fiber.Ctx) error {
	var cfg struct{ Enabled bool }
	if err := c.Bind().Body(&cfg); err != nil {
		return err
	}
	return nil
}

func unrelatedHandler(n int) string { return strconv.Itoa(n) }
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "gate_probe.go", src, 0)
	if err != nil {
		t.Fatalf("parse probe source: %v", err)
	}
	funcs := map[string]*ast.FuncDecl{}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			funcs[fn.Name.Name] = fn
		}
	}
	want := map[string]bodyBinder{
		"strictHandler":       binderStrict,
		"strictDecodeHandler": binderStrict,
		"permissiveHandler":   binderPermissive,
		"unrelatedHandler":    binderUnknown,
	}
	for name, expect := range want {
		if got := handlerBinder(funcs[name]); got != expect {
			t.Errorf("handlerBinder(%s) = %v, want %v", name, got, expect)
		}
	}
}
