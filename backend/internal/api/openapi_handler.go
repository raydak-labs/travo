package api

import (
	"encoding/json"

	"github.com/gofiber/fiber/v3"
)

// openAPISpec is the OpenAPI 3.0 specification for the openwrt-travel-gui backend.
// Served at GET /api/openapi.json for agent/test automation use.
var openAPISpec = map[string]any{
	"openapi": "3.0.3",
	"info": map[string]any{
		"title":       "OpenWRT Travel Router GUI API",
		"description": "REST API for managing an OpenWRT travel router",
		"version":     "1.0.0",
	},
	"servers": []map[string]any{
		{"url": "/api/v1", "description": "Local device API"},
	},
	"components": map[string]any{
		"securitySchemes": map[string]any{
			"bearerAuth": map[string]any{
				"type":         "http",
				"scheme":       "bearer",
				"bearerFormat": "JWT",
			},
		},
	},
	"security": []map[string]any{
		{"bearerAuth": []string{}},
	},
	"paths": mergePathMaps(
		openAPIPathsAuth,
		openAPIPathsSystem,
		openAPIPathsNetwork,
		openAPIPathsWifi,
		openAPIPathsVpn,
		openAPIPathsServices,
		openAPIPathsSqm,
		openAPIPathsAdguard,
		openAPIPathsCaptive,
	),
}

// mergePathMaps combines the per-domain path maps into the spec's "paths"
// object. The spec is split by domain so that a file per area of the API
// holds the routes of that area, instead of one literal with every route in
// it.
func mergePathMaps(domains ...map[string]any) map[string]any {
	paths := map[string]any{}
	for _, domain := range domains {
		for path, operations := range domain {
			paths[path] = operations
		}
	}
	return paths
}

// endpoint builds an OpenAPI operation object. The optional parameters are
// path (param) and query (query) parameter objects, in that order.
func endpoint(operationID, summary string, requiresAuth bool, requestBody, response map[string]any, parameters ...map[string]any) map[string]any {
	op := map[string]any{
		"operationId": operationID,
		"summary":     summary,
		"responses":   map[string]any{"200": response},
	}
	if requiresAuth {
		op["security"] = []map[string]any{{"bearerAuth": []string{}}}
	} else {
		op["security"] = []map[string]any{}
	}
	if requestBody != nil {
		op["requestBody"] = requestBody
	}
	if len(parameters) > 0 {
		op["parameters"] = parameters
	}
	return op
}

// param builds a required path parameter object.
func param(name, description string) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "path",
		"required":    true,
		"description": description,
		"schema":      map[string]any{"type": "string"},
	}
}

// query builds an optional query parameter object.
func query(name, description string) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "query",
		"required":    false,
		"description": description,
		"schema":      map[string]any{"type": "string"},
	}
}

// body builds a requestBody object.
func body(contentType string, example map[string]any) map[string]any {
	content := map[string]any{}
	if example != nil {
		content[contentType] = map[string]any{
			"schema": map[string]any{"type": "object", "example": example},
		}
	} else {
		content[contentType] = map[string]any{}
	}
	return map[string]any{"required": true, "content": content}
}

// resp200 builds a 200 response object.
func resp200(contentType string, example map[string]any) map[string]any {
	content := map[string]any{}
	if example != nil {
		content[contentType] = map[string]any{
			"schema": map[string]any{"type": "object", "example": example},
		}
	} else {
		content[contentType] = map[string]any{}
	}
	return map[string]any{"description": "OK", "content": content}
}

// wifiApplyEnvelope is the 200 body every wireless mutator answers, built by
// wifiMutationResponse in wifi_handlers.go: {"status":"ok","apply":{…}} where
// apply carries the pending token the browser must confirm and the rollback
// timeout. probe_budget_seconds is how long ONE confirm call can block on the
// device while it waits for the new interfaces to come up; a client that re-POSTs
// confirm until the rollback deadline has to leave that much room, or a probe it
// starts near the deadline is answered after rpcd has already rolled back. extra
// adds the one endpoint-specific key a handler appends (allow_ap_on_sta_radio, mac).
func wifiApplyEnvelope(extra map[string]any) map[string]any {
	body := map[string]any{
		"status": "ok",
		"apply": map[string]any{
			"pending":                  true,
			"token":                    "",
			"rollback_timeout_seconds": 90,
			"probe_budget_seconds":     4,
		},
	}
	for k, v := range extra {
		body[k] = v
	}
	return body
}

// obj builds a simple string-keyed example object where all values are empty strings.
func obj(keys ...string) map[string]any {
	m := make(map[string]any, len(keys))
	for _, k := range keys {
		m[k] = ""
	}
	return m
}

// openAPIJSON is the cached JSON encoding of openAPISpec.
var openAPIJSON []byte

func init() {
	b, err := json.Marshal(openAPISpec)
	if err != nil {
		panic("openapi: failed to marshal spec: " + err.Error())
	}
	openAPIJSON = b
}

// OpenAPIHandler serves the OpenAPI 3.0 specification as JSON.
func OpenAPIHandler() fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Set("Content-Type", "application/json")
		return c.Send(openAPIJSON)
	}
}
