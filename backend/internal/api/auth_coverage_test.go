package api

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// publicRouteSet indexes PublicRoutes for the coverage test below.
func publicRouteSet() map[string]bool {
	set := make(map[string]bool, len(PublicRoutes))
	for p := range PublicRoutes {
		set[p] = true
	}
	return set
}

// caseVariant upper-cases a path so the request cannot match the registered
// route when routing is case-sensitive, while still exercising any
// string-matching on the raw request path.
func caseVariant(path string) string {
	segs := strings.Split(path, "/")
	for i, s := range segs {
		if s != "" {
			segs[i] = strings.ToUpper(s)
		}
	}
	return strings.Join(segs, "/")
}

// sampleValue replaces a Fiber :param segment so a canonical-spelling request
// addresses a route that could actually match.
func sampleValue(path string) string {
	segs := strings.Split(path, "/")
	for i, s := range segs {
		if strings.HasPrefix(s, ":") {
			switch s {
			case ":index":
				segs[i] = "0"
			default:
				segs[i] = "sample"
			}
		}
	}
	return strings.Join(segs, "/")
}

// notHandlerRun are the statuses that prove neither the middleware nor the
// handler executed. 401 means the auth middleware rejected the request; 404/405
// mean the router never matched a route for it. Any other status means
// application code ran, which is what the coverage test exists to prevent for
// unauthenticated callers.
func notHandlerRun(status int) bool {
	switch status {
	case http.StatusUnauthorized, http.StatusNotFound, http.StatusMethodNotAllowed:
		return true
	}
	return false
}

// TestAuthCoversEveryRoute is the gate for the public/protected split. It walks
// the live route table rather than a hand-maintained list, so adding a route
// without deciding whether it is public fails here instead of shipping.
//
// For every registered /api route it asserts an unauthenticated request is
// rejected — in the canonical spelling and in a case-varied spelling — unless
// the route is in PublicRoutes.
func TestAuthCoversEveryRoute(t *testing.T) {
	app, _ := setupTestApp(t)

	public := publicRouteSet()
	checked := 0

	for _, r := range app.GetRoutes() {
		path := r.Path
		if !strings.HasPrefix(path, "/api/") {
			continue
		}
		// Fiber auto-registers 405 stubs on the bare group prefix. They have
		// no handler of their own and are not routes anyone can call.
		if path == "/api/v1" {
			continue
		}
		checked++

		canonical := sampleValue(path)
		variants := []struct {
			label string
			path  string
			// strict requires exactly 401: the middleware must run and reject.
			strict bool
		}{
			{"canonical", canonical, true},
			{"case variant", caseVariant(canonical), false},
		}

		for _, v := range variants {
			req, err := http.NewRequest(r.Method, v.path, nil)
			if err != nil {
				t.Fatalf("building request for %s %s: %v", r.Method, v.path, err)
			}
			resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
			if err != nil {
				t.Fatalf("request %s %s failed: %v", r.Method, v.path, err)
			}
			status := resp.StatusCode
			resp.Body.Close()

			if public[path] {
				if status == http.StatusUnauthorized {
					t.Errorf("route %s %s is listed in PublicRoutes but answers 401 without a token",
						r.Method, path)
				}
				continue
			}

			if v.strict {
				if status != http.StatusUnauthorized {
					t.Errorf("route %s %s answered %d to an unauthenticated request, want 401 — "+
						"it must be mounted on the authenticated group in SetupRoutes",
						r.Method, path, status)
				}
				continue
			}

			if !notHandlerRun(status) {
				t.Errorf("route %s %s (%s, %q) answered %d to an unauthenticated request — "+
					"application code ran without a token",
					r.Method, path, v.label, v.path, status)
			}
		}
	}

	// Guard against the walk silently covering nothing (see the same
	// anti-vacuity guard in openapi_drift_test.go).
	if checked < 100 {
		t.Fatalf("only %d /api routes were checked — the walk is probably broken", checked)
	}
}

// TestPublicRoutesAreReachable pins the other half of the contract: the listed
// public paths must actually be registered, so a rename cannot silently leave a
// public entry pointing at nothing.
func TestPublicRoutesAreReachable(t *testing.T) {
	app, _ := setupTestApp(t)

	registered := map[string]bool{}
	for _, r := range app.GetRoutes() {
		registered[r.Path] = true
	}

	// /api/v1/ws is registered by main.go, not SetupRoutes, so it is absent
	// from the test app by design.
	for p := range PublicRoutes {
		if p == "/api/v1/ws" {
			continue
		}
		if !registered[p] {
			t.Errorf("PublicRoutes lists %q but no route with that path is registered", p)
		}
	}
}

// TestPublicEndpointsAreReachable is the regression guard for a bug this
// coverage test's own structure would not have caught: the auth middleware was
// briefly mounted on the /api/v1 route group, and because Fiber v3 scopes group
// middleware by PATH PREFIX rather than by which router a route was registered
// on, it also intercepted /api/v1/ws and /api/v1/system/time-sync -- breaking
// the WebSocket and the pre-login clock recovery while every "route is
// protected" assertion still passed.
func TestPublicEndpointsAreReachable(t *testing.T) {
	app, _ := setupTestApp(t)

	// A token would be required if these were treated as protected.
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/system/time-sync", nil)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("time-sync request failed: %v", err)
	}
	status := resp.StatusCode
	resp.Body.Close()
	if status == http.StatusUnauthorized {
		t.Error("POST /api/v1/system/time-sync answered 401: the pre-login clock " +
			"recovery path is unauthenticated by design and must stay reachable")
	}

	req, _ = http.NewRequest(http.MethodGet, "/api/health", nil)
	resp, err = app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	status = resp.StatusCode
	resp.Body.Close()
	if status != http.StatusOK {
		t.Errorf("GET /api/health answered %d, want 200", status)
	}
}
