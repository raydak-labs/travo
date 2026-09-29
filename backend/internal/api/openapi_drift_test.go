package api

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
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
	app, deps := setupTestApp()
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
