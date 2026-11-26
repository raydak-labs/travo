package api

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/reboot"
)

const rebootRequestPath = "/api/v1/system/reboot"

type fakeRebooter struct {
	calls    int
	accepted reboot.Accepted
	err      error
}

func (f *fakeRebooter) Reboot(context.Context) (reboot.Accepted, error) {
	f.calls++
	return f.accepted, f.err
}

func newRebootEnv(t *testing.T, fake *fakeRebooter) *env {
	t.Helper()
	return newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) {
		if fake != nil {
			d.Rebooter = fake
		}
		d.BootID = func(context.Context) string { return "opaque-current-boot" }
	})
}

func TestRebootRequiresAuthAndCSRF(t *testing.T) {
	fake := &fakeRebooter{accepted: reboot.Accepted{BootID: "before"}}
	e := newRebootEnv(t, fake)
	rec := e.do(req{method: http.MethodPost, path: rebootRequestPath, body: `{"confirm":true}`})
	if rec.Code != http.StatusUnauthorized || errCode(t, rec) != "unauthenticated" {
		t.Fatalf("without cookie: %d %s", rec.Code, rec.Body)
	}
	cookie, _ := e.login()
	rec = e.do(req{method: http.MethodPost, path: rebootRequestPath, body: `{"confirm":true}`, cookie: cookie})
	if rec.Code != http.StatusForbidden || errCode(t, rec) != "csrf" {
		t.Fatalf("without csrf: %d %s", rec.Code, rec.Body)
	}
	if fake.calls != 0 {
		t.Fatalf("service calls = %d", fake.calls)
	}
}

func TestRebootRejectsInvalidBodyWithoutCallingService(t *testing.T) {
	for _, body := range []string{`{}`, `{"confirm":"yes"}`, ``, `{"confirm":true,"extra":1}`} {
		t.Run(body, func(t *testing.T) {
			fake := &fakeRebooter{}
			e := newRebootEnv(t, fake)
			cookie, csrf := e.login()
			rec := e.do(req{method: http.MethodPost, path: rebootRequestPath, body: body, cookie: cookie, csrf: csrf})
			if rec.Code != http.StatusBadRequest || errCode(t, rec) != "invalid_request" {
				t.Fatalf("%d %s", rec.Code, rec.Body)
			}
			if fake.calls != 0 {
				t.Fatalf("service calls = %d", fake.calls)
			}
		})
	}
}

func TestRebootRequiresConfirmation(t *testing.T) {
	fake := &fakeRebooter{}
	e := newRebootEnv(t, fake)
	cookie, csrf := e.login()
	rec := e.do(req{method: http.MethodPost, path: rebootRequestPath, body: `{"confirm":false}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusBadRequest || errCode(t, rec) != "confirmation_required" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if fake.calls != 0 {
		t.Fatalf("service calls = %d", fake.calls)
	}
}

func TestRebootAccepted(t *testing.T) {
	fake := &fakeRebooter{accepted: reboot.Accepted{BootID: "opaque-before"}}
	e := newRebootEnv(t, fake)
	cookie, csrf := e.login()
	rec := e.do(req{method: http.MethodPost, path: rebootRequestPath, body: `{"confirm":true}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusAccepted || bodyField(t, rec, "status") != "ok" || bodyField(t, rec, "boot_id") != "opaque-before" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if fake.calls != 1 {
		t.Fatalf("service calls = %d", fake.calls)
	}
}

func TestRebootServiceErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "pending", err: reboot.ErrPending, status: http.StatusConflict, code: "reboot_pending"},
		{name: "transaction", err: reboot.ErrTxnPending, status: http.StatusConflict, code: "transaction_pending"},
		{name: "update", err: reboot.ErrUpdateBusy, status: http.StatusConflict, code: "update_busy"},
		{name: "failed", err: reboot.ErrFailed, status: http.StatusInternalServerError, code: "reboot_failed"},
		{name: "unexpected", err: errors.New("unexpected"), status: http.StatusInternalServerError, code: "reboot_failed"},
		{name: "unavailable", err: reboot.ErrUnavailable, status: http.StatusServiceUnavailable, code: "reboot_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeRebooter{err: tc.err}
			e := newRebootEnv(t, fake)
			cookie, csrf := e.login()
			rec := e.do(req{method: http.MethodPost, path: rebootRequestPath, body: `{"confirm":true}`, cookie: cookie, csrf: csrf})
			if rec.Code != tc.status || errCode(t, rec) != tc.code {
				t.Fatalf("%d %s", rec.Code, rec.Body)
			}
			if fake.calls != 1 {
				t.Fatalf("service calls = %d", fake.calls)
			}
		})
	}
}

func TestRebootNilServiceIsUnavailable(t *testing.T) {
	e := newRebootEnv(t, nil)
	cookie, csrf := e.login()
	rec := e.do(req{method: http.MethodPost, path: rebootRequestPath, body: `{"confirm":true}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusServiceUnavailable || errCode(t, rec) != "reboot_unavailable" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestHealthIncludesOpaqueBootIDAndRemainsPublicOffLAN(t *testing.T) {
	e := newRebootEnv(t, nil)
	rec := e.do(req{method: http.MethodGet, path: "/api/health", remote: "203.0.113.10:12345"})
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if bodyField(t, rec, "boot_id") != "opaque-current-boot" || bodyField(t, rec, "version") != "1.2.3" {
		t.Fatalf("health = %s", rec.Body)
	}
}

func TestHealthReadsLiveBootID(t *testing.T) {
	current := "before"
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) {
		d.BootID = func(context.Context) string { return current }
	})
	if got := bodyField(t, e.do(req{method: http.MethodGet, path: "/api/health"}), "boot_id"); got != "before" {
		t.Fatalf("boot id = %q", got)
	}
	current = "after"
	if got := bodyField(t, e.do(req{method: http.MethodGet, path: "/api/health"}), "boot_id"); got != "after" {
		t.Fatalf("boot id after rotation = %q", got)
	}
}
