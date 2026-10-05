package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/openwrt-travel-gui/backend/internal/services"
)

func TestSystemInfoEndpoint(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/system/info", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["hostname"]; !ok {
		t.Error("expected hostname in response")
	}
}

func TestSystemStatsEndpoint(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/system/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["cpu"]; !ok {
		t.Error("expected cpu in response")
	}
}

func TestReboot_ReturnsOk(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/system/reboot", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d, body: %s", resp.StatusCode, b)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "ok") {
		t.Errorf("expected ok in response, got: %s", body)
	}
}

func TestFactoryReset_ReturnsError(t *testing.T) {
	// Factory reset calls exec.Command("firstboot") which won't exist in test env,
	// so we expect a 500 error.
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/system/factory-reset", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	// firstboot doesn't exist in test environment, so expect 500
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 (firstboot not available in test), got %d", resp.StatusCode)
	}
}

func TestFirmwareUpgrade_NoFile(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/system/firmware/upgrade", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestFirmwareUpgrade_InvalidExtension(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("firmware", "firmware.txt")
	_, _ = part.Write([]byte("fake firmware data"))
	_ = writer.Close()

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/system/firmware/upgrade", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid extension, got %d", resp.StatusCode)
	}
}

// buildFirmwareImage returns a synthetic sysupgrade image: kernel padding plus
// an OpenWrt metadata block, in the layout mkimage writes.
func buildFirmwareImage(t *testing.T, model string, supportedDevices []string) []byte {
	t.Helper()
	var pairs bytes.Buffer
	pair := func(k, v string) {
		pairs.WriteString(k)
		pairs.WriteByte(0)
		pairs.WriteString(v)
		pairs.WriteByte(0)
	}
	pair("version", "23.05.2")
	pair("distname", "OpenWrt")
	pair("model", model)
	pair("supported_devices", strings.Join(supportedDevices, ","))

	const magic = "metadata\x00\x00"
	body := append([]byte(magic), 0, 0, 0, 1)
	body = append(body, pairs.Bytes()...)
	block := make([]byte, 0, 8+len(body))
	block = binary.BigEndian.AppendUint32(block, uint32(8+len(body)))
	block = binary.BigEndian.AppendUint32(block, 1)
	block = append(block, body...)

	out := make([]byte, 4096) // kernel padding in front of the block
	for i := range out {
		out[i] = 0xff
	}
	return append(out, block...)
}

// The mock ubus answers board_name "glinet,gl-mt3000" / model "GL.iNet GL-MT3000".
func mockBoardDevices() []string { return []string{"gl-mt3000", "glinet,gl-mt3000"} }

// installFakeSysupgrade puts a stub sysupgrade first on PATH that records the
// arguments of every invocation, so a test can prove a rejected image or
// archive was never handed to it.
func installFakeSysupgrade(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	marker := filepath.Join(dir, "sysupgrade-calls")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + marker + "\n"
	if err := os.WriteFile(filepath.Join(dir, "sysupgrade"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake sysupgrade: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return marker
}

func sysupgradeCalls(t *testing.T, marker string) []string {
	t.Helper()
	b, err := os.ReadFile(marker)
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}

// waitForSysupgradeCall blocks until the asynchronous flash has run.
func waitForSysupgradeCall(t *testing.T, marker string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(marker); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("sysupgrade was never invoked (%s missing)", marker)
}

func multipartUpload(t *testing.T, field, filename string, content []byte,
	fields map[string]string,
) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return body, writer.FormDataContentType()
}

func postMultipart(t *testing.T, app *fiber.App, token, path string,
	body *bytes.Buffer, contentType string,
) (int, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", contentType)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// tarGz builds a gzip tar archive from headers. A member's body is taken from
// bodies[name] when present, otherwise Size zero bytes are written
// (archive/tar enforces that the declared size is actually written).
func tarGz(t *testing.T, headers []*tar.Header, bodies map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, h := range headers {
		if body, ok := bodies[h.Name]; ok {
			h.Size = int64(len(body))
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatalf("write tar header %q: %v", h.Name, err)
		}
		if h.Typeflag != tar.TypeReg || h.Size <= 0 {
			continue
		}
		if body, ok := bodies[h.Name]; ok {
			if _, err := io.WriteString(tw, body); err != nil {
				t.Fatalf("write tar body %q: %v", h.Name, err)
			}
			continue
		}
		if _, err := io.CopyN(tw, zeroReader{}, h.Size); err != nil {
			t.Fatalf("write tar filler %q: %v", h.Name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

// zeroReader feeds archive/tar the declared member size without allocating it.
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

// genuineBackupArchive is what `sysupgrade -b` produces: the UCI config
// directory and nothing else.
func genuineBackupArchive(t *testing.T) []byte {
	t.Helper()
	return tarGz(t,
		[]*tar.Header{
			{Name: "etc", Typeflag: tar.TypeDir, Mode: 0o755},
			{Name: "etc/config", Typeflag: tar.TypeDir, Mode: 0o755},
			{Name: "etc/config/network", Typeflag: tar.TypeReg, Mode: 0o600},
			{Name: "etc/config/wireless", Typeflag: tar.TypeReg, Mode: 0o600},
		},
		map[string]string{
			"etc/config/network":  "config network 'lan'\n",
			"etc/config/wireless": "config wifi-iface 'default'\n",
		},
	)
}

func TestFirmwareUpgrade_ValidFile(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")
	deps.System.SetGuardDir(t.TempDir())
	// The flash goroutine must finish inside this test, or it would pick up a
	// later test's PATH and record a call that has nothing to do with it.
	marker := installFakeSysupgrade(t)

	body, ct := multipartUpload(t, "firmware", "openwrt-sysupgrade.bin",
		buildFirmwareImage(t, "GL.iNet GL-MT3000", mockBoardDevices()),
		map[string]string{"keep_settings": "true"})

	code, respBody := postMultipart(t, app, token, "/api/v1/system/firmware/upgrade", body, ct)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", code, respBody)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(respBody), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	// The parsed model is returned so the UI can show what is being flashed.
	if got["model"] != "GL.iNet GL-MT3000" {
		t.Errorf("model = %v, want GL.iNet GL-MT3000", got["model"])
	}
	if _, ok := got["supported_devices"]; !ok {
		t.Errorf("the response must carry supported_devices: %s", respBody)
	}
	waitForSysupgradeCall(t, marker)
}

// A .bin for a different board was accepted and answered 200 "upgrade
// initiated", and the flash only failed afterwards with sysupgrade's own error —
// on a router with no serial console. It must be a 400 that names the model,
// and sysupgrade must never see the image.
func TestFirmwareUpgrade_RejectsForeignDevice(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")
	marker := installFakeSysupgrade(t)

	body, ct := multipartUpload(t, "firmware", "other-router.bin",
		buildFirmwareImage(t, "Some Other Router", []string{"other,router-x1"}), nil)

	code, respBody := postMultipart(t, app, token, "/api/v1/system/firmware/upgrade", body, ct)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a foreign image, got %d: %s", code, respBody)
	}
	if !strings.Contains(respBody, "Some Other Router") {
		t.Errorf("the 400 must name the image model: %s", respBody)
	}
	// The flash runs in a goroutine after a 500ms delay.
	time.Sleep(900 * time.Millisecond)
	if calls := sysupgradeCalls(t, marker); len(calls) > 0 {
		t.Errorf("sysupgrade was invoked for a rejected image: %v", calls)
	}
}

// A junk file with a .bin name is the textbook brick: no metadata block, so
// nothing to verify. It must be refused before the flash starts.
func TestFirmwareUpgrade_RejectsJunkBinary(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")
	marker := installFakeSysupgrade(t)

	body, ct := multipartUpload(t, "firmware", "not-a-firmware.bin",
		bytes.Repeat([]byte{0x41}, 4096), nil)

	code, respBody := postMultipart(t, app, token, "/api/v1/system/firmware/upgrade", body, ct)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a junk .bin, got %d: %s", code, respBody)
	}
	time.Sleep(900 * time.Millisecond)
	if calls := sysupgradeCalls(t, marker); len(calls) > 0 {
		t.Errorf("sysupgrade was invoked for a junk image: %v", calls)
	}
}

// PUT /system/leds documents a single boolean and drives every LED. On the
// permissive binder a body naming the field the spec USED to advertise
// ("enabled") decoded to stealth_mode=false and answered 200 with the LEDs
// unchanged: the opposite of the request, behind a success code.
func TestSetLEDStealth_RejectsWrongFieldName(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	code, body := putJSON(t, app, token, "/api/v1/system/leds", `{"enabled":true}`)
	if code != http.StatusBadRequest {
		t.Errorf("PUT /system/leds {\"enabled\":true} returned %d, want 400: %s", code, body)
	}
	if !strings.Contains(body, "unknown field") || !strings.Contains(body, "enabled") {
		t.Errorf("the 400 must name the field the request used: %s", body)
	}
}

func TestSetLEDStealth_AcceptsStealthMode(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	code, body := putJSON(t, app, token, "/api/v1/system/leds", `{"stealth_mode":true}`)
	// Off-device there are no LEDs under /sys/class/leds, so the handler either
	// answers with the (empty) status or fails writing it — but it must not
	// reject the documented field name.
	if code == http.StatusBadRequest {
		t.Fatalf("the documented body was rejected: %s", body)
	}
}

// The upload used to be written to the FIXED path /tmp/restore-upload.tar.gz
// in world-writable /tmp: two concurrent restores overwrote each other's
// payload, so the archive sysupgrade extracted was not necessarily the one
// that was validated. Each restore must hand sysupgrade its own file.
func TestRestore_ConcurrentUploadsUseDistinctFiles(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")
	deps.System.SetGuardDir(t.TempDir())
	marker := installFakeSysupgrade(t)

	const n = 3
	type upload struct {
		body *bytes.Buffer
		ct   string
	}
	uploads := make([]upload, n)
	for i := range uploads {
		b, ct := multipartUpload(t, "backup", "backup.tar.gz", genuineBackupArchive(t), nil)
		uploads[i] = upload{body: b, ct: ct}
	}

	type result struct {
		code int
		body string
		err  error
	}
	results := make(chan result, n)
	var wg sync.WaitGroup
	for i := range uploads {
		wg.Add(1)
		go func(u upload) {
			defer wg.Done()
			req, _ := http.NewRequest(http.MethodPost, "/api/v1/system/restore", u.body)
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", u.ct)
			resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
			if err != nil {
				results <- result{err: err}
				return
			}
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			results <- result{code: resp.StatusCode, body: string(b)}
		}(uploads[i])
	}
	wg.Wait()
	close(results)
	for r := range results {
		if r.err != nil {
			t.Fatalf("concurrent restore request failed: %v", r.err)
		}
		if r.code != http.StatusOK {
			t.Errorf("a concurrent restore returned %d: %s", r.code, r.body)
		}
	}

	calls := sysupgradeCalls(t, marker)
	if len(calls) != n {
		t.Fatalf("sysupgrade was called %d times, want %d: %v", len(calls), n, calls)
	}
	seen := make(map[string]bool, n)
	for _, call := range calls {
		fields := strings.Fields(call)
		if len(fields) < 2 {
			t.Fatalf("unexpected sysupgrade invocation %q", call)
		}
		path := fields[len(fields)-1]
		if path == "/tmp/restore-upload.tar.gz" {
			t.Errorf("the restore was handed the old fixed path: %q", call)
		}
		if !strings.HasPrefix(filepath.Base(path), "restore-upload-") {
			t.Errorf("unexpected restore temp path %q", path)
		}
		if seen[path] {
			t.Errorf("two restores shared the file %q", path)
		}
		seen[path] = true
	}
}

// A restore archive is extracted at / by sysupgrade -r, so a member outside the
// config allowlist is an arbitrary root code-execution path. It must be a 400
// that never reaches sysupgrade.
// TestRestore_RejectsArchiveThatIsNotABackup covers the one membership rule
// that remains: an archive must carry at least one UCI config member.
//
// There is deliberately no directory allowlist beyond that. `sysupgrade -b`
// archives the overlay upper layer and this application itself writes
// /etc/crontabs/root, /etc/dropbear/authorized_keys and /etc/shadow, so
// restricting members to etc/config and etc/ppp refused every genuine backup
// taken on this device. Authorization is the boundary for restore (ADR 0007
// section 2), and an authenticated admin already holds root through
// POST /system/ssh-keys, firmware flash and factory reset.
func TestRestore_RejectsArchiveThatIsNotABackup(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")
	marker := installFakeSysupgrade(t)

	evil := tarGz(t,
		[]*tar.Header{
			{Name: "etc/crontabs/root", Typeflag: tar.TypeReg, Mode: 0o600},
		},
		map[string]string{"etc/crontabs/root": "* * * * * id\n"},
	)
	body, ct := multipartUpload(t, "backup", "backup.tar.gz", evil, nil)

	code, respBody := postMultipart(t, app, token, "/api/v1/system/restore", body, ct)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an archive with no UCI config member, got %d: %s", code, respBody)
	}
	if !strings.Contains(respBody, "not a configuration backup") {
		t.Errorf("the 400 must explain why: %s", respBody)
	}
	if calls := sysupgradeCalls(t, marker); len(calls) > 0 {
		t.Errorf("sysupgrade was invoked for a rejected archive: %v", calls)
	}
}

// TestRestore_AcceptsOverlayBackupWithAppWrittenFiles is the handler-level
// counterpart of the same decision: a real backup from this device carries the
// files Travo itself writes, and restore must not refuse it.
func TestRestore_AcceptsOverlayBackupWithAppWrittenFiles(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")
	marker := installFakeSysupgrade(t)

	backup := tarGz(t,
		[]*tar.Header{
			{Name: "etc/config/network", Typeflag: tar.TypeReg, Mode: 0o600},
			{Name: "etc/crontabs/root", Typeflag: tar.TypeReg, Mode: 0o600},
			{Name: "etc/dropbear/authorized_keys", Typeflag: tar.TypeReg, Mode: 0o600},
			{Name: "etc/shadow", Typeflag: tar.TypeReg, Mode: 0o600},
		},
		map[string]string{"etc/crontabs/root": "* * * * * id\n"},
	)
	body, ct := multipartUpload(t, "backup", "backup.tar.gz", backup, nil)

	code, respBody := postMultipart(t, app, token, "/api/v1/system/restore", body, ct)
	if code != http.StatusOK {
		t.Fatalf("a genuine overlay backup was refused with %d: %s", code, respBody)
	}
	if calls := sysupgradeCalls(t, marker); len(calls) != 1 {
		t.Errorf("sysupgrade should have been invoked exactly once, got %v", calls)
	}
}

func TestRestore_RejectsSymlinkMember(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")
	marker := installFakeSysupgrade(t)

	evil := tarGz(t,
		[]*tar.Header{
			{Name: "etc/config/link", Typeflag: tar.TypeSymlink, Linkname: "/etc/init.d/travo", Size: 0},
		}, nil)
	body, ct := multipartUpload(t, "backup", "backup.tar.gz", evil, nil)

	code, respBody := postMultipart(t, app, token, "/api/v1/system/restore", body, ct)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a symlink member, got %d: %s", code, respBody)
	}
	if calls := sysupgradeCalls(t, marker); len(calls) > 0 {
		t.Errorf("sysupgrade was invoked for a rejected archive: %v", calls)
	}
}

// The restore handler must report the service's rejection as a 400 (the
// operator uploaded the wrong file), not as a 500.
func TestRestore_ReportsRejectionAsBadRequest(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	body, ct := multipartUpload(t, "backup", "backup.tar.gz", []byte("this is not a tarball"), nil)
	code, respBody := postMultipart(t, app, token, "/api/v1/system/restore", body, ct)
	if code != http.StatusBadRequest {
		t.Errorf("expected 400 for a non-archive upload, got %d: %s", code, respBody)
	}
	if !strings.Contains(respBody, services.ErrInvalidBackupArchive.Error()) {
		t.Errorf("the response should explain why the archive was rejected: %s", respBody)
	}
}

func TestGetNTPConfig(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/system/ntp", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["enabled"]; !ok {
		t.Error("expected enabled in response")
	}
	if _, ok := data["servers"]; !ok {
		t.Error("expected servers in response")
	}
}

func TestSetNTPConfig(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	payload := `{"enabled":true,"servers":["pool.ntp.org","time.google.com"]}`
	req, _ := http.NewRequest(http.MethodPut, "/api/v1/system/ntp", strings.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d, body: %s", resp.StatusCode, b)
	}
}

func TestGetSetupComplete(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/system/setup-complete", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0, FailOnTimeout: false})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d, body: %s", resp.StatusCode, b)
	}
	body, _ := io.ReadAll(resp.Body)
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["complete"]; !ok {
		t.Error("expected complete in response")
	}
}

// A body that does not match the documented shape used to be accepted and the
// zero value persisted. Both handlers write whole config objects, so both must
// bind strictly.
func TestSystemConfigPUTs_RejectUnknownField(t *testing.T) {
	app, deps := setupTestApp(t)
	token, _, _ := deps.Auth.Login("admin")

	cases := []struct {
		path string
		body string
	}{
		{"/api/v1/system/hostname", `{"host_name":"x"}`},
		{"/api/v1/system/button-actions", `{"buttonz":{}}`},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			code, body := putJSON(t, app, token, tc.path, tc.body)
			if code != http.StatusBadRequest {
				t.Errorf("expected 400 for an unknown field, got %d: %s", code, body)
			}
		})
	}
}
