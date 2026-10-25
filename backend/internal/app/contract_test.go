package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/api"
	"github.com/openwrt-travel-gui/backend/internal/config"
)

// skipContract lists GETs whose body is not a JSON document (streams, the
// backup archive) or that run a measurement instead of reading state.
var skipContract = regexp.MustCompile(`/ws$|/stream|/backup$|/speedtest`)

// notInMock answers with a documented error in mock mode, where AdGuard Home
// is not installed.
var notInMock = map[string]int{"/api/v1/adguard/config": http.StatusNotFound}

// Every GET the frontend can call must answer with a body that matches its
// own OpenAPI schema: the generated client trusts the schema blindly, so a
// field the schema omits, a null where it promises an array, or a value
// outside an enum is a runtime crash in the UI rather than a type error.
func TestMockResponsesMatchTheirSchemas(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MockMode = true
	cfg.AuthConfigPath = filepath.Join(t.TempDir(), "auth.json")
	app, lifecycle := mustNew(t, cfg)
	lifecycle.Stop()
	token := mockLogin(t, app)

	doc := api.SetupRoutes(fiber.New(), &api.Dependencies{}).OpenAPI()
	registry := doc.Components.Schemas

	paths := make([]string, 0, len(doc.Paths))
	for p := range doc.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	checked := 0
	for _, p := range paths {
		op := doc.Paths[p].Get
		if op == nil || strings.Contains(p, "{") || skipContract.MatchString(p) {
			continue
		}
		resp := op.Responses["200"]
		if resp == nil || resp.Content["application/json"] == nil {
			continue
		}
		schema := resp.Content["application/json"].Schema

		t.Run(p, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, p, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			res, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if want, ok := notInMock[p]; ok && res.StatusCode == want {
				return
			}
			if res.StatusCode != http.StatusOK {
				var e map[string]any
				_ = json.NewDecoder(res.Body).Decode(&e)
				t.Fatalf("status %d: %v", res.StatusCode, e)
			}
			var body any
			if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			var result huma.ValidateResult
			huma.Validate(registry, schema, huma.NewPathBuffer(nil, 0), huma.ModeReadFromServer, body, &result)
			for _, e := range result.Errors {
				t.Error(e)
			}
		})
		checked++
	}
	if checked < 50 {
		t.Fatalf("only %d GET operations checked; the filter is too broad", checked)
	}
}

func mockLogin(t *testing.T, app *fiber.App) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte(`{"password":"admin"}`)))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var login struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(res.Body).Decode(&login); err != nil || login.Token == "" {
		t.Fatalf("mock login: %v", err)
	}
	return login.Token
}
