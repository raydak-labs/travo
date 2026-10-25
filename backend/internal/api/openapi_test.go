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

// spec fetches the served OpenAPI document.
func spec(t *testing.T, app *fiber.App) map[string]any {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, OpenAPIPath, nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("GET %s: %v", OpenAPIPath, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %d", OpenAPIPath, resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("spec is not JSON: %v", err)
	}
	return doc
}

func specOps(t *testing.T, doc map[string]any) map[string]map[string]any {
	t.Helper()
	ops := map[string]map[string]any{}
	for path, item := range doc["paths"].(map[string]any) {
		for method, op := range item.(map[string]any) {
			ops[strings.ToUpper(method)+" "+path] = op.(map[string]any)
		}
	}
	return ops
}

// Every REST route must be a Huma operation: a raw Fiber route under /api/v1
// would be served without appearing in the contract.
func TestEveryRouteIsInTheSpec(t *testing.T) {
	app, _ := setupTestApp(t)
	ops := specOps(t, spec(t, app))
	var missing []string
	for _, r := range app.GetRoutes() {
		if !strings.HasPrefix(r.Path, "/api/v1/") || r.Method == http.MethodHead || r.Path == "/api/v1/ws" {
			continue
		}
		path := r.Path
		for _, seg := range strings.Split(r.Path, "/") {
			if strings.HasPrefix(seg, ":") {
				path = strings.Replace(path, seg, "{"+seg[1:]+"}", 1)
			}
		}
		if _, ok := ops[r.Method+" "+path]; !ok {
			missing = append(missing, r.Method+" "+path)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("routes missing from the spec:\n  %s", strings.Join(missing, "\n  "))
	}
	if len(ops) < 150 {
		t.Errorf("spec has only %d operations; registration is probably incomplete", len(ops))
	}
}

func TestOperationsAreIdentifiedAndDescribed(t *testing.T) {
	app, _ := setupTestApp(t)
	seen := map[string]string{}
	for where, op := range specOps(t, spec(t, app)) {
		id, _ := op["operationId"].(string)
		if id == "" || strings.TrimSpace(op["summary"].(string)) == "" {
			t.Errorf("%s needs an operationId and a summary", where)
		}
		if other, dup := seen[id]; dup {
			t.Errorf("%s and %s share operationId %q", where, other, id)
		}
		seen[id] = where
	}
}

// A PUT that persists a multi-field object must reject unknown fields, or a
// body with a misspelled field decodes to zero values and overwrites the
// user's settings behind a 200 (decodeStrictJSON).
func TestWholeConfigPutsDecodeStrictly(t *testing.T) {
	permissive := map[string]string{
		"PUT /api/v1/auth/password":         "a credential change, not a config object",
		"PUT /api/v1/adguard/password":      "a credential change, not a config object",
		"PUT /api/v1/adguard/config":        "a single raw YAML document",
		"PUT /api/v1/wifi/ap/{section}":     "a partial update of one access point",
		"PUT /api/v1/wifi/repeater-options": "a single policy flag",
		"PUT /api/v1/wifi/mac":              "a single MAC",
	}
	app, _ := setupTestApp(t)
	doc := spec(t, app)
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	checked := 0
	for where, op := range specOps(t, doc) {
		if !strings.HasPrefix(where, "PUT ") {
			continue
		}
		rb, ok := op["requestBody"].(map[string]any)
		if !ok {
			continue
		}
		schema := rb["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
		if ref, ok := schema["$ref"].(string); ok {
			schema = schemas[strings.TrimPrefix(ref, "#/components/schemas/")].(map[string]any)
		}
		props, _ := schema["properties"].(map[string]any)
		if len(props) < 2 {
			continue
		}
		checked++
		strict, _ := op["x-strict-body"].(bool)
		if reason, known := permissive[where]; known {
			if strict {
				t.Errorf("%s decodes strictly now; drop it from the permissive list", where)
			}
			_ = reason
			continue
		}
		if !strict {
			t.Errorf("%s replaces a %d-field object but does not decode strictly; use strict()", where, len(props))
		}
	}
	if checked < 10 {
		t.Fatalf("only %d whole-config PUTs were checked; the gate is probably broken", checked)
	}
}
