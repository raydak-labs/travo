package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/apitokens"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/firmware"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

func firmwareEnv(t *testing.T) (*env, guard.Dir) {
	t.Helper()
	g := guard.Dir{Path: t.TempDir(), Now: time.Now}
	store := apitokens.New(filepath.Join(t.TempDir(), "api-tokens.json"))
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) {
		d.Tokens = store
		dev := d.Ubus.(*mock.Device)
		mock.SeedSystem(dev)
		// The verbatim capture validates a zero file; only its verdict is flipped.
		raw, err := os.ReadFile("../../testdata/container/openwrt-25.12.6/sysupgrade/validate-zero-image.json")
		if err != nil {
			t.Fatal(err)
		}
		dev.FirmwareValidation = bytes.Replace(raw, []byte(`"valid": false`), []byte(`"valid": true`), 1)
		d.Firmware = &firmware.Service{Device: dev, Guards: g}
	})
	return e, g
}

func imageForm(t *testing.T, content string) (body, contentType string) {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, err := w.CreateFormFile("image", "openwrt-sysupgrade.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.String(), w.FormDataContentType()
}

func TestFirmwareUploadThenGuardedApply(t *testing.T) {
	e, g := firmwareEnv(t)
	cookie, csrf := e.login()
	body, ct := imageForm(t, "image bytes")
	rec := e.do(req{method: "POST", path: "/api/v1/system/firmware", body: body, contentType: ct, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body)
	}
	var img firmware.Image
	if err := json.Unmarshal(rec.Body.Bytes(), &img); err != nil {
		t.Fatal(err)
	}
	if img.Size != 11 || !img.Validation.Valid || img.Board != "glinet,gl-axt1800" || !img.PackagesKnown {
		t.Fatalf("%+v", img)
	}
	rec = e.do(req{method: "GET", path: "/api/v1/system/firmware", cookie: cookie})
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(img.SHA256)) {
		t.Fatalf("status: %d %s", rec.Code, rec.Body)
	}
	apply := func(confirm string) *bytes.Buffer {
		b, _ := json.Marshal(map[string]any{"sha256": img.SHA256, "keep_settings": true, "confirm": confirm})
		r := e.do(req{method: "POST", path: "/api/v1/system/firmware/apply", body: string(b), cookie: cookie, csrf: csrf})
		if confirm != firmware.Phrase && r.Code != http.StatusBadRequest {
			t.Fatalf("%q: %d", confirm, r.Code)
		}
		if confirm == firmware.Phrase && r.Code != http.StatusAccepted {
			t.Fatalf("apply: %d %s", r.Code, r.Body)
		}
		return r.Body
	}
	apply("yes")
	if len(e.dev.Sysupgrades) != 0 {
		t.Fatal("flashed without the phrase")
	}
	apply(firmware.Phrase)
	if len(e.dev.Sysupgrades) != 1 || !e.dev.Sysupgrades[0] {
		t.Fatalf("sysupgrades %v", e.dev.Sysupgrades)
	}
	if held, _ := g.Stale(); len(held) != 1 || held[0].Name != firmware.GuardName {
		t.Fatalf("guards %v", held)
	}
	rec = e.do(req{method: "POST", path: "/api/v1/system/firmware", body: body, contentType: ct, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusConflict || errCode(t, rec) != "firmware_upgrade_pending" {
		t.Fatalf("upload while pending: %d %s", rec.Code, rec.Body)
	}
}

func TestFirmwareApplyRefusedWhileAnotherGuardIsHeld(t *testing.T) {
	e, g := firmwareEnv(t)
	cookie, csrf := e.login()
	body, ct := imageForm(t, "x")
	rec := e.do(req{method: "POST", path: "/api/v1/system/firmware", body: body, contentType: ct, cookie: cookie, csrf: csrf})
	var img firmware.Image
	if err := json.Unmarshal(rec.Body.Bytes(), &img); err != nil {
		t.Fatal(err)
	}
	if err := g.Acquire("reboot-in-progress"); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(map[string]any{"sha256": img.SHA256, "keep_settings": false, "confirm": firmware.Phrase})
	rec = e.do(req{method: "POST", path: "/api/v1/system/firmware/apply", body: string(b), cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusConflict || errCode(t, rec) != "operation_in_progress" || len(e.dev.Sysupgrades) != 0 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestFirmwareUploadRefusesOversizedBodyBeforeReading(t *testing.T) {
	e, _ := firmwareEnv(t)
	cookie, csrf := e.login()
	body, ct := imageForm(t, "x")
	for length, code := range map[int64]string{firmware.MaxImageBytes + 1<<20: "firmware_too_large", -1: "length_required"} {
		rec := e.do(req{method: "POST", path: "/api/v1/system/firmware", body: body, contentType: ct, contentLength: length, cookie: cookie, csrf: csrf})
		if errCode(t, rec) != code {
			t.Fatalf("%d: %d %s", length, rec.Code, rec.Body)
		}
	}
	// Not enough RAM for the declared size: refused before the body lands in /tmp.
	e.dev.SetUbusReply("system", "info", []byte(`{"uptime":1,"load":[1,2,3],"memory":{"total":100000000,"available":40000000},"root":{"total":8,"avail":4},"tmp":{"avail":200000}}`))
	rec := e.do(req{method: "POST", path: "/api/v1/system/firmware", body: body, contentType: ct, contentLength: 20 << 20, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusInsufficientStorage || errCode(t, rec) != "firmware_no_room" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestNoTokenMayUploadOrFlashFirmware(t *testing.T) {
	e, _ := firmwareEnv(t)
	_, admin := e.mint("admin")
	body, ct := imageForm(t, "x")
	for _, r := range []req{
		{method: "GET", path: "/api/v1/system/firmware"},
		{method: "POST", path: "/api/v1/system/firmware", body: body, contentType: ct},
		{method: "POST", path: "/api/v1/system/firmware/apply", body: `{"sha256":"` + string(bytes.Repeat([]byte("a"), 64)) + `","keep_settings":true,"confirm":"upgrade firmware"}`},
	} {
		r.auth = "Bearer " + admin
		rec := e.do(r)
		if rec.Code != http.StatusForbidden || errCode(t, rec) != "session_required" {
			t.Fatalf("%s %s: %d %s", r.method, r.path, rec.Code, rec.Body)
		}
	}
	if len(e.dev.Sysupgrades) != 0 {
		t.Fatal("flashed")
	}
}
