package osupdate

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
)

type fakePackages struct {
	updated   bool
	updateErr error
	listErr   error
}

func (*fakePackages) Name() string { return "apk" }
func (f *fakePackages) Update(context.Context) error {
	f.updated = true
	return f.updateErr
}
func (f *fakePackages) Upgradable(context.Context) ([]pkgmgr.Upgrade, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return []pkgmgr.Upgrade{{Name: "dnsmasq", Installed: "2.93-r1", Available: "2.93-r2"}}, nil
}

func newService(t *testing.T, index []byte, pkgs *fakePackages) (*Service, *mock.Device, *[]string) {
	t.Helper()
	dev := mock.New()
	mock.SeedSystem(dev)
	dev.DefaultRoute = true
	urls := &[]string{}
	s := &Service{Ubus: dev, Routes: dev, Packages: pkgs, Now: func() time.Time { return time.Unix(1795000000, 0) },
		Fetch: func(_ context.Context, url string) ([]byte, error) {
			*urls = append(*urls, url)
			return index, nil
		}}
	return s, dev, urls
}

func TestCheckReportsReleasesAndPackages(t *testing.T) {
	pkgs := &fakePackages{}
	s, _, urls := newService(t, mock.VersionsIndex, pkgs)
	if l := s.Last(); l.CheckedAt != nil || l.Packages == nil {
		t.Fatalf("before first check: %+v", l)
	}
	r, err := s.Check(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if (*urls)[0] != "https://downloads.openwrt.org/.versions.json" {
		t.Fatalf("url %q", (*urls)[0])
	}
	if r.Current != "25.12.6" || r.InSeries != "" || r.Target != "qualcommax/ipq60xx" || pkgs.updated || !r.PackagesRead || len(r.Packages) != 1 {
		t.Fatalf("%+v", r)
	}
	if l := s.Last(); l.CheckedAt == nil || !l.CheckedAt.Equal(time.Unix(1795000000, 0)) {
		t.Fatalf("last %+v", l)
	}
}

func TestCheckRefreshIsBestEffort(t *testing.T) {
	pkgs := &fakePackages{updateErr: errors.New("no route to feeds")}
	s, _, _ := newService(t, mock.VersionsIndex, pkgs)
	r, err := s.Check(context.Background(), true)
	if err != nil || !pkgs.updated || r.ListsRefreshed || !r.PackagesRead {
		t.Fatalf("%+v %v", r, err)
	}
	pkgs = &fakePackages{listErr: errors.New("apk broke")}
	s, _, _ = newService(t, mock.VersionsIndex, pkgs)
	r, err = s.Check(context.Background(), true)
	if err != nil || !r.ListsRefreshed || r.PackagesRead || r.Packages == nil {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestCheckOfflineWithoutRoute(t *testing.T) {
	s, dev, urls := newService(t, mock.VersionsIndex, &fakePackages{})
	dev.DefaultRoute = false
	if _, err := s.Check(context.Background(), false); !errors.Is(err, ErrOffline) {
		t.Fatal(err)
	}
	if len(*urls) != 0 {
		t.Fatal("fetched without a route")
	}
}

func TestCheckRejectsBadIndex(t *testing.T) {
	s, _, _ := newService(t, []byte("<html>"), &fakePackages{})
	if _, err := s.Check(context.Background(), false); !errors.Is(err, ErrFailed) {
		t.Fatal(err)
	}
	if s.Last().CheckedAt != nil {
		t.Fatal("failed check stored")
	}
}

func TestHTTPFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			_, _ = w.Write([]byte("{}"))
		case "/big":
			_, _ = w.Write([]byte(strings.Repeat("x", IndexLimit+1)))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	fetch := HTTPFetch(srv.Client())
	ctx := context.Background()
	if b, err := fetch(ctx, srv.URL+"/ok"); err != nil || string(b) != "{}" {
		t.Fatal(b, err)
	}
	for path, want := range map[string]error{"/big": ErrFailed, "/missing": ErrFailed} {
		if _, err := fetch(ctx, srv.URL+path); !errors.Is(err, want) {
			t.Errorf("%s: %v", path, err)
		}
	}
	srv.Close()
	if _, err := fetch(ctx, srv.URL+"/ok"); !errors.Is(err, ErrOffline) {
		t.Fatal(err)
	}
}
