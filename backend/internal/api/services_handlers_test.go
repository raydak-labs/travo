package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/services"
)

// The service endpoints take no request body, so none of their errors can be a
// malformed body — and the UI shows the message verbatim. Prefixing them with
// "invalid request body" told the operator their request was wrong when the real
// cause was a missing service or a failed opkg/initd call.
//
// This pins the message for both shapes of failure: an unknown id and a
// service that is known but not installed. The status code is deliberately left
// at 400 — separating "you asked for something that does not exist" from "the
// device failed to start it" needs an error type the service manager does not
// have yet, and that re-classification is not part of this change.
func TestServiceHandlerErrorsAreNotLabelledAsBodyErrors(t *testing.T) {
	sm := services.NewServiceManagerWith(services.NewMockPackageManager(), &services.MockSystemProbe{})

	app := fiber.New()
	app.Post("/api/v1/services/:id/start", StartServiceHandler(sm))
	app.Post("/api/v1/services/:id/install", InstallServiceHandler(sm))

	for _, tc := range []struct {
		name     string
		path     string
		wantText string
	}{
		{"unknown service", "/api/v1/services/not-a-service/start", "service not found"},
		{"known but not installed", "/api/v1/services/adguardhome/start", "not installed"},
		{"install of unknown service", "/api/v1/services/not-a-service/install", "service not found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			var payload struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode %s: %v", body, err)
			}
			if strings.Contains(payload.Error, ErrInvalidRequestBody) {
				t.Errorf("error is labelled as a body error although the request has no body: %q", payload.Error)
			}
			if !strings.Contains(payload.Error, tc.wantText) {
				t.Errorf("error %q does not name the real cause (want %q)", payload.Error, tc.wantText)
			}
		})
	}
}
