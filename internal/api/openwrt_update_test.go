package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/osupdate"
)

type fakeOpenWrt struct {
	err     error
	refresh *bool
}

func (fakeOpenWrt) Last() osupdate.OpenWrtCheck { return osupdate.OpenWrtCheck{Packages: nil} }
func (f fakeOpenWrt) Check(_ context.Context, refresh bool) (osupdate.OpenWrtCheck, error) {
	if f.refresh != nil {
		*f.refresh = refresh
	}
	if f.err != nil {
		return osupdate.OpenWrtCheck{}, f.err
	}
	r := osupdate.OpenWrtCheck{Target: "qualcommax/ipq60xx"}
	r.Current, r.InSeries = "25.12.3", "25.12.6"
	return r, nil
}

func TestOpenWrtUpdateCheck(t *testing.T) {
	var refresh bool
	e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.OpenWrtUpdates = fakeOpenWrt{refresh: &refresh} })
	cookie, csrf := e.login()
	rec := e.do(req{method: http.MethodGet, path: "/api/v1/system/openwrt", cookie: cookie})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"checked_at":null`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	rec = e.do(req{method: http.MethodPost, path: "/api/v1/system/openwrt/check", body: `{"refresh_lists":true}`, cookie: cookie, csrf: csrf})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"in_series":"25.12.6"`) || !refresh {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}

func TestOpenWrtUpdateCheckErrors(t *testing.T) {
	for err, code := range map[error]string{
		osupdate.ErrOffline: "update_check_offline",
		errors.Join(osupdate.ErrFailed, errors.New("x")): "update_check_failed",
	} {
		e := newEnvWithDashboard(t, nil, nil, nil, func(d *Deps) { d.OpenWrtUpdates = fakeOpenWrt{err: err} })
		cookie, csrf := e.login()
		rec := e.do(req{method: http.MethodPost, path: "/api/v1/system/openwrt/check", body: `{}`, cookie: cookie, csrf: csrf})
		if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), code) {
			t.Errorf("%v: %d %s", err, rec.Code, rec.Body)
		}
	}
	e := newEnv(t)
	cookie, _ := e.login()
	if rec := e.do(req{method: http.MethodGet, path: "/api/v1/system/openwrt", cookie: cookie}); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("%d", rec.Code)
	}
}
