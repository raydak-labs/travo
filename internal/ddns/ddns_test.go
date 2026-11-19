package ddns

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
)

const captures = "../../testdata/device/axt1800-25.12.6/ddns/"

func capture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(captures + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// fresh is the package's default config: global plus two disabled samples, no travo.
func fresh(t *testing.T) []device.Section {
	t.Helper()
	secs, err := cli.ParseExport(Config, string(capture(t, "uci-export-ddns.txt")))
	if err != nil {
		t.Fatal(err)
	}
	return secs
}

var duck = Request{Enabled: true, Service: "duckdns.org", Domain: "trip.duckdns.org", Password: "tok-1"}

func TestReadAndValidate(t *testing.T) {
	v, err := Read(fresh(t))
	if err != nil || v.Enabled || v.PasswordSet {
		t.Fatalf("fresh = %+v, %v (the samples are not Travo's)", v, err)
	}
	foreign := append(fresh(t), device.Section{Name: Section, Type: "ddns"})
	if _, err := Read(foreign); !errors.Is(err, ErrForeign) {
		t.Errorf("foreign: %v", err)
	}
	svcs := []string{"duckdns.org"}
	if err := Validate(duck, svcs, false); err != nil {
		t.Fatal(err)
	}
	for field, in := range map[string]Request{
		"service":  {Service: "evil.example", Domain: "a.b.cd", Password: "x"},
		"domain":   {Service: "duckdns.org", Domain: "-bad", Password: "x"},
		"username": {Service: "duckdns.org", Domain: "a.duckdns.org", Username: "a\nb", Password: "x"},
		"password": {Service: "duckdns.org", Domain: "a.duckdns.org"},
	} {
		var fe *FieldError
		if err := Validate(in, svcs, false); !errors.As(err, &fe) || fe.Field != field {
			t.Errorf("%s: %v", field, err)
		}
	}
	if err := Validate(Request{Service: "duckdns.org", Domain: "a.duckdns.org"}, svcs, true); err != nil {
		t.Errorf("stored token kept: %v", err)
	}
}

func setup(t *testing.T) (*Service, *mock.Device, *pkgmgr.Mock) {
	t.Helper()
	d := mock.New()
	d.SetService(InitScript, false, false)
	root := t.TempDir()
	dir := filepath.Join(root, ServicesDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "duckdns.org.json"), capture(t, "service-duckdns.org.json"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkgs := pkgmgr.NewMock()
	s := &Service{UCI: d, Keys: d, Packages: pkgs, Services: d, Txns: applier{d},
		Guards:    guard.Dir{Path: filepath.Join(t.TempDir(), "guards"), Now: time.Now},
		FreeBytes: func() (uint64, error) { return 60 << 20, nil },
		Online:    func(context.Context) bool { return true }, Root: root}
	return s, d, pkgs
}

// applier stages and commits like txn.Apply, reverting on a stage error.
type applier struct{ d *mock.Device }

func (a applier) Apply(ctx context.Context, configs []string, stage func(context.Context) error) error {
	if err := stage(ctx); err != nil {
		return errors.Join(err, a.d.Revert(ctx, configs[0]))
	}
	return a.d.Commit(ctx, configs[0])
}

func TestInstallThenSet(t *testing.T) {
	s, d, _ := setup(t)
	if v, err := s.Settings(t.Context()); err != nil || v.Installed {
		t.Fatalf("absent = %+v, %v", v, err)
	}
	if err := s.Set(t.Context(), duck); !errors.Is(err, pkgmgr.ErrNotInstalled) {
		t.Fatalf("set before install: %v", err)
	}
	s.FreeBytes = func() (uint64, error) { return 4 << 20, nil }
	if err := s.Install(t.Context()); !errors.Is(err, ErrNoSpace) {
		t.Fatalf("no space: %v", err)
	}
	s.FreeBytes = func() (uint64, error) { return 60 << 20, nil }
	if err := s.Install(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(d.ServiceActions, "ddns enable") {
		t.Errorf("actions %v", d.ServiceActions)
	}
	if err := d.Load(t.Context(), Config, fresh(t)); err != nil { // what the package ships
		t.Fatal(err)
	}
	if err := s.Set(t.Context(), duck); err != nil {
		t.Fatal(err)
	}
	v, err := s.Settings(t.Context())
	if err != nil || !v.Installed || !v.Enabled || !v.PasswordSet || v.Domain != duck.Domain || !slices.Equal(v.Services, []string{"duckdns.org"}) {
		t.Fatalf("after set = %+v, %v", v, err)
	}
	secs, _ := d.Show(t.Context(), Config)
	tr := secs[slices.IndexFunc(secs, func(s device.Section) bool { return s.Name == Section })]
	if tr.Options["password"] != "tok-1" || tr.Options["interface"] != Uplink || tr.Options["use_https"] != "1" || tr.Options["lookup_host"] != duck.Domain {
		t.Errorf("travo = %+v", tr.Options)
	}
	if d.ServiceActions[len(d.ServiceActions)-1] != "ddns restart" {
		t.Errorf("no restart: %v", d.ServiceActions)
	}
	same := duck
	same.Password = ""
	if err := s.Set(t.Context(), same); !errors.Is(err, ErrNoChange) {
		t.Errorf("unchanged: %v", err)
	}
	same.Enabled = false
	if err := s.Set(t.Context(), same); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.Settings(t.Context()); v.Enabled || !v.PasswordSet {
		t.Errorf("disable kept the token? %+v", v)
	}
}

func TestRegisteredIPAndBusy(t *testing.T) {
	s, d, pkgs := setup(t)
	_ = pkgs.Install(t.Context(), Package)
	_ = d.Load(t.Context(), Config, fresh(t))
	ip := filepath.Join(s.Root, IPFile)
	_ = os.MkdirAll(filepath.Dir(ip), 0o755)
	_ = os.WriteFile(ip, []byte("203.0.113.7\n"), 0o644)
	if v, _ := s.Settings(t.Context()); v.RegisteredIP != "203.0.113.7" {
		t.Errorf("ip = %q", v.RegisteredIP)
	}
	_ = os.WriteFile(ip, []byte("garbage"), 0o644)
	if v, _ := s.Settings(t.Context()); v.RegisteredIP != "" {
		t.Errorf("garbage ip = %q", v.RegisteredIP)
	}
	if err := s.Guards.Acquire(InstallGuard); err != nil {
		t.Fatal(err)
	}
	if err := s.Set(t.Context(), duck); !errors.Is(err, ErrBusy) {
		t.Errorf("busy: %v", err)
	}
}

func TestViewNeverCarriesToken(t *testing.T) {
	s, d, pkgs := setup(t)
	_ = pkgs.Install(t.Context(), Package)
	_ = d.Load(t.Context(), Config, fresh(t))
	if err := s.Set(t.Context(), duck); err != nil {
		t.Fatal(err)
	}
	v, _ := s.Settings(t.Context())
	if strings.Contains(strings.Join([]string{v.Service, v.Domain, v.Username, v.RegisteredIP}, " "), "tok-1") {
		t.Error("token leaked into the view")
	}
}
