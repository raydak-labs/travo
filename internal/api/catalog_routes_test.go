package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/catalog"
)

type fakeCatalog struct {
	err   error
	calls []string
}

func (f *fakeCatalog) List(context.Context) (catalog.View, error) {
	return catalog.View{Entries: []catalog.Item{{ID: "vnstat", Packages: []string{"vnstat2"}, Installed: true}}}, nil
}

func (f *fakeCatalog) Install(_ context.Context, id string) error {
	f.calls = append(f.calls, "install "+id)
	return f.err
}

func (f *fakeCatalog) Remove(_ context.Context, id string) error {
	f.calls = append(f.calls, "remove "+id)
	return f.err
}

func (f *fakeCatalog) SetAutostart(_ context.Context, id string, on bool) error {
	if on {
		f.calls = append(f.calls, "on "+id)
	} else {
		f.calls = append(f.calls, "off "+id)
	}
	return f.err
}

func TestCatalogRoutes(t *testing.T) {
	f := &fakeCatalog{}
	e := newEnvWithDashboard(t, nil, nil, nil, func(deps *Deps) { deps.Catalog = f })
	cookie, csrf := e.login()
	if rec := e.do(req{method: "GET", path: "/api/v1/services/catalog", cookie: cookie}); rec.Code != http.StatusOK ||
		!strings.Contains(rec.Body.String(), `"id":"vnstat"`) {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}
	for _, r := range []req{
		{method: "POST", path: "/api/v1/services/catalog/vnstat/install"},
		{method: "POST", path: "/api/v1/services/catalog/vnstat/remove"},
		{method: "PUT", path: "/api/v1/services/catalog/vnstat/autostart", body: `{"enabled":true}`},
	} {
		r.cookie, r.csrf = cookie, csrf
		if rec := e.do(r); rec.Code != http.StatusOK {
			t.Fatalf("%s %s: %d %s", r.method, r.path, rec.Code, rec.Body)
		}
	}
	if strings.Join(f.calls, ",") != "install vnstat,remove vnstat,on vnstat" {
		t.Fatalf("calls %v", f.calls)
	}
	for code, err := range map[string]error{
		"not_found":             catalog.ErrUnknown,
		"catalog_busy":          catalog.ErrBusy,
		"catalog_offline":       catalog.ErrOffline,
		"catalog_no_space":      catalog.ErrNoSpace,
		"catalog_built_in":      catalog.ErrBuiltIn,
		"catalog_in_use":        catalog.ErrInUse,
		"catalog_not_installed": catalog.ErrNotInstalled,
		"catalog_unavailable":   errors.New("apk exploded"),
	} {
		f.err = err
		rec := e.do(req{method: "POST", path: "/api/v1/services/catalog/vnstat/install", cookie: cookie, csrf: csrf})
		if !strings.Contains(rec.Body.String(), `"code":"`+code+`"`) {
			t.Errorf("%s: %d %s", code, rec.Code, rec.Body)
		}
	}
}
