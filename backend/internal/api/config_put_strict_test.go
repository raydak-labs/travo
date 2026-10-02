package api

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/services"
)

func putJSON(t *testing.T, app *fiber.App, token, path, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPut, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// GET /wifi/band-switching answers {"config":{…},"status":{…}} and PUT used to
// bind the bare struct. The natural client — read the documented GET shape,
// change a field, PUT it back — therefore decoded to the zero value and the
// handler answered 200 after overwriting every setting with zeros. Found on the
// device: preferred_band, both thresholds and both delays all became 0.
func TestSetBandSwitching_AcceptsGetWrapperShape(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body := `{"config":{"enabled":false,"preferred_band":"2g","check_interval_sec":10,` +
		`"down_switch_threshold_dbm":-70,"down_switch_delay_sec":30,` +
		`"up_switch_threshold_dbm":-60,"up_switch_delay_sec":60,"min_viable_signal_dbm":-80},` +
		`"status":{"state":"inactive","current_band":"","signal_dbm":0,"weak_signal_secs":0,"cooldown_sec":0}}`
	code, respBody := putJSON(t, app, token, "/api/v1/wifi/band-switching", body)
	if code != http.StatusOK {
		t.Fatalf("PUT with the GET wrapper shape returned %d: %s", code, respBody)
	}

	got := deps.BandSwitching.GetConfig()
	if got.PreferredBand != "2g" || got.DownSwitchThresholdDBm != -70 ||
		got.UpSwitchDelaySec != 60 || got.MinViableSignalDBm != -80 {
		t.Errorf("the wrapper body was not applied: %+v", got)
	}
}

// The bare shape must keep working: the frontend and existing scripts use it.
func TestSetBandSwitching_AcceptsBareShape(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	code, body := putJSON(t, app, token, "/api/v1/wifi/band-switching",
		`{"enabled":false,"preferred_band":"5g","check_interval_sec":10,`+
			`"down_switch_threshold_dbm":-70,"down_switch_delay_sec":30,`+
			`"up_switch_threshold_dbm":-60,"up_switch_delay_sec":60,"min_viable_signal_dbm":-80}`)
	if code != http.StatusOK {
		t.Fatalf("PUT with the bare shape returned %d: %s", code, body)
	}
	if got := deps.BandSwitching.GetConfig(); got.PreferredBand != "5g" {
		t.Errorf("the bare body was not applied: %+v", got)
	}
}

// A wrong-shaped body must be a 400, not a silent wipe. This is the whole point
// of BindStrictBodyConfig.
func TestSetBandSwitching_RejectsUnknownField(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")
	before := deps.BandSwitching.GetConfig()

	code, body := putJSON(t, app, token, "/api/v1/wifi/band-switching",
		`{"enabled":true,"preferred_band":"5g","check_interval_sec":10,`+
			`"down_switch_threshold_dbm":-70,"down_switch_delay_sec":30,`+
			`"up_switch_threshold_dbm":-60,"up_switch_delay_sec":60,"min_viable_signal_dbm":-80,`+
			`"typo_field":1}`)
	if code != http.StatusBadRequest {
		t.Errorf("expected 400 for an unknown field, got %d: %s", code, body)
	}
	if got := deps.BandSwitching.GetConfig(); got != before {
		t.Errorf("a rejected request still changed the config: before %+v, after %+v", before, got)
	}
}

// preferred_band accepted "9g" and answered 200, which persists a band the
// switcher can never match — so it silently does nothing.
func TestSetBandSwitching_ValidatesBand(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	for _, bad := range []string{`"9g"`, `"5G"`, `"wifi6"`} {
		code, body := putJSON(t, app, token, "/api/v1/wifi/band-switching",
			`{"enabled":true,"preferred_band":`+bad+`,"check_interval_sec":10,`+
				`"down_switch_threshold_dbm":-70,"down_switch_delay_sec":30,`+
				`"up_switch_threshold_dbm":-60,"up_switch_delay_sec":60,"min_viable_signal_dbm":-80}`)
		if code != http.StatusBadRequest {
			t.Errorf("preferred_band=%s returned %d, want 400: %s", bad, code, body)
		}
	}
	if got := deps.BandSwitching.GetConfig(); got.PreferredBand == "9g" {
		t.Error("an invalid band was persisted")
	}
}

// storage_percent=500 was accepted and persisted: a threshold no usage level can
// reach, so the alert never fires while the UI shows a saved configuration.
func TestSetAlertThresholds_RejectsOutOfRange(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")
	// Off-device the store lives in /etc/travo, which is not writable here.
	deps.Alerts.SetThresholdsFile(t.TempDir() + "/alert-thresholds.json")

	code, body := putJSON(t, app, token, "/api/v1/system/alert-thresholds",
		`{"storage_percent":500,"cpu_percent":90,"memory_percent":90}`)
	if code != http.StatusBadRequest {
		t.Errorf("expected 400 for storage_percent=500, got %d: %s", code, body)
	}
	code, _ = putJSON(t, app, token, "/api/v1/system/alert-thresholds",
		`{"storage_percent":-1,"cpu_percent":90,"memory_percent":90}`)
	if code != http.StatusBadRequest {
		t.Errorf("expected 400 for storage_percent=-1, got %d", code)
	}
	code, _ = putJSON(t, app, token, "/api/v1/system/alert-thresholds",
		`{"storage_percent":90,"cpu_percent":90,"memory_percent":90}`)
	if code != http.StatusOK {
		t.Errorf("a valid body returned %d, want 200", code)
	}
	if got := deps.Alerts.GetAlertThresholds(); got.StoragePercent != 90 {
		t.Errorf("the valid body was not applied: %+v", got)
	}
}

// A negative check_interval_sec was accepted. It happens not to crash (SetConfig
// coerces it to the default), but persisting a value the service will not honour
// leaves the UI showing a setting that is not in effect.
func TestSetBandSwitching_RejectsNonPositiveInterval(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	code, body := putJSON(t, app, token, "/api/v1/wifi/band-switching",
		`{"enabled":true,"preferred_band":"5g","check_interval_sec":-5,`+
			`"down_switch_threshold_dbm":-70,"down_switch_delay_sec":30,`+
			`"up_switch_threshold_dbm":-60,"up_switch_delay_sec":60,"min_viable_signal_dbm":-80}`)
	if code != http.StatusBadRequest {
		t.Errorf("expected 400 for check_interval_sec=-5, got %d: %s", code, body)
	}
	if got := deps.BandSwitching.GetConfig(); got.CheckIntervalSec == -5 {
		t.Error("a rejected interval was persisted")
	}
}

// dBm hysteresis: coming back up needs a STRONGER signal than going down, i.e. a
// larger (less negative) number. The defaults are -70 down / -60 up.
func TestValidateBandSwitchConfig_Hysteresis(t *testing.T) {
	base := services.BandSwitchConfig{
		PreferredBand:          "5g",
		CheckIntervalSec:       10,
		DownSwitchThresholdDBm: -70,
		DownSwitchDelaySec:     30,
		UpSwitchThresholdDBm:   -60,
		UpSwitchDelaySec:       60,
		MinViableSignalDBm:     -80,
	}
	if err := services.ValidateBandSwitchConfig(base); err != nil {
		t.Errorf("the shipped defaults must validate: %v", err)
	}

	bad := base
	bad.UpSwitchThresholdDBm = -80 // weaker than the down threshold
	if err := services.ValidateBandSwitchConfig(bad); err == nil {
		t.Error("up_switch below down_switch must be rejected: the switcher would oscillate")
	}
	bad = base
	bad.DownSwitchThresholdDBm = 40 // not a dBm value
	if err := services.ValidateBandSwitchConfig(bad); err == nil {
		t.Error("a positive dBm threshold must be rejected")
	}
}

// BindStrictBodyConfig itself: unknown fields, wrong JSON kinds, and trailing
// content all have to be errors.
func TestBindStrictBodyConfig(t *testing.T) {
	type target struct {
		Name  string   `json:"name"`
		Count int      `json:"count"`
		List  []string `json:"list"`
	}
	cases := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"valid", `{"name":"a","count":1,"list":["x"]}`, false},
		// `null` decodes into a non-pointer struct as a no-op: no error, dst
		// untouched, so the caller would persist the zero value and answer 200.
		{"null", `null`, true},
		{"null with whitespace", "\n  null\n", true},
		{"unknown field", `{"name":"a","nope":1}`, true},
		{"wrong kind", `{"list":"not-a-list"}`, true},
		{"wrong kind for int", `{"count":"one"}`, true},
		{"trailing content", `{"name":"a"}{"name":"b"}`, true},
		{"not an object", `[1,2,3]`, true},
		{"malformed", `{"name":`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			var got target
			app.Put("/", func(c fiber.Ctx) error {
				if err := BindStrictBodyConfig(c, &got); err != nil {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
				}
				return c.SendStatus(fiber.StatusOK)
			})
			req, _ := http.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte(tc.body)))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			defer resp.Body.Close()
			if tc.wantErr && resp.StatusCode == fiber.StatusOK {
				t.Errorf("expected a rejection, got 200 (decoded %+v)", got)
			}
			if !tc.wantErr && resp.StatusCode != fiber.StatusOK {
				b, _ := io.ReadAll(resp.Body)
				t.Errorf("expected 200, got %d: %s", resp.StatusCode, b)
			}
		})
	}
}

// The wrapper form must be unwrapped before strict decoding, so an unknown field
// inside the envelope is still caught.
func TestDecodeStrictJSON_Unwraps(t *testing.T) {
	type inner struct {
		Band string `json:"band"`
	}
	var got inner
	if err := decodeStrictJSON([]byte(`{"config":{"band":"5g"},"status":{}}`), &got, "config"); err != nil {
		t.Fatalf("valid wrapper rejected: %v", err)
	}
	if got.Band != "5g" {
		t.Errorf("wrapper not unwrapped: %+v", got)
	}
	if err := decodeStrictJSON([]byte(`{"config":{"band":"5g","oops":1}}`), &got, "config"); err == nil {
		t.Error("an unknown field inside the wrapper must be rejected")
	}
	if err := decodeStrictJSON([]byte(`{"other":{}}`), &got, "config"); err == nil {
		t.Error("a body without the wrapper key must be rejected")
	}
}
