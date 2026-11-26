package portfwd

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

const captures = "../../testdata/device/axt1800-25.12.6/"

func read(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(captures + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// stockExamples uncomments the two `config redirect` examples of the stock
// /etc/config/firewall (portfwd/rom-firewall-redirect-examples.txt): the shape LuCI and
// hand-written configs use, never created by Travo. The examples' unquoted, tab-aligned values
// are re-quoted the way `uci export` prints them.
func stockExamples(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, l := range strings.Split(read(t, "portfwd/rom-firewall-redirect-examples.txt"), "\n") {
		f := strings.Fields(strings.TrimPrefix(l, "#"))
		switch {
		case strings.HasPrefix(l, "#config"):
			b.WriteString(strings.Join(f, " ") + "\n")
		case strings.HasPrefix(l, "#\t") && len(f) == 3:
			b.WriteString("\t" + f[0] + " " + f[1] + " '" + f[2] + "'\n")
		}
	}
	return b.String()
}

// firewall is the captured `uci export firewall` (no redirects) plus, when foreign is set,
// the stock examples appended.
func firewall(t *testing.T, foreign bool) []device.Section {
	t.Helper()
	text := read(t, "network/uci-export-firewall.txt")
	if foreign {
		text += "\n" + stockExamples(t)
	}
	secs, err := cli.ParseExport("firewall", text)
	if err != nil {
		t.Fatal(err)
	}
	return secs
}

type fakeTxns struct {
	d       *mock.Device
	summary string
	active  bool
}

func (f *fakeTxns) Begin(ctx context.Context, summary string, configs []string, stage func(context.Context) error, _ txn.Probe) (txn.Status, error) {
	if f.active {
		return txn.Status{}, txn.ErrActive
	}
	if !reflect.DeepEqual(configs, []string{"firewall"}) {
		return txn.Status{}, errors.New("unexpected configs")
	}
	f.summary = summary
	if err := stage(ctx); err != nil {
		return txn.Status{}, errors.Join(err, f.d.Revert(ctx, "firewall"))
	}
	if err := f.d.Commit(ctx, "firewall"); err != nil {
		return txn.Status{}, err
	}
	return txn.Status{Meta: txn.Meta{ID: "0123456789abcdef"}, State: txn.Pending}, nil
}

func setup(t *testing.T, foreign bool) (*Service, *mock.Device, *fakeTxns) {
	t.Helper()
	d := mock.New()
	network, err := cli.ParseShow("network", read(t, "clients/uci-show-network-lan.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Load(t.Context(), "network", network); err != nil {
		t.Fatal(err)
	}
	if err := d.Load(t.Context(), "firewall", firewall(t, foreign)); err != nil {
		t.Fatal(err)
	}
	f := &fakeTxns{d: d}
	return &Service{UCI: d, Txns: f}, d, f
}

func TestListStockRedirectsAsForeign(t *testing.T) {
	got := List(firewall(t, true))
	want := []Forward{
		{ID: got[0].ID, Proto: "tcp", ExternalPort: "80", TargetIP: "192.168.16.235", TargetPort: "80", Src: "wan", Dest: "lan", Target: "DNAT", Enabled: true},
		{ID: got[1].ID, Proto: "tcp", ExternalPort: "22001", TargetPort: "22", Src: "wan", Dest: "lan", Target: "DNAT", Enabled: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
	if l := List(firewall(t, false)); len(l) != 0 {
		t.Fatalf("captured firewall has no redirects: %+v", l)
	}
}

func TestParsePorts(t *testing.T) {
	for in, want := range map[string]PortRange{"80": {80, 80}, "8000-8010": {8000, 8010}, "1:65535": {1, 65535}} {
		if got, ok := ParsePorts(in); !ok || got != want {
			t.Errorf("%q: %v %v", in, got, ok)
		}
	}
	for _, bad := range []string{"", "0", "65536", "10-5", "a", "-1", "1-", "+80", "000080", "80-90-100"} {
		if _, ok := ParsePorts(bad); ok {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestValidate(t *testing.T) {
	lan := netip.MustParsePrefix("192.168.1.1/24")
	v, err := Validate(ForwardRequest{Proto: "both", ExternalPort: "8000-8010", TargetIP: "192.168.1.50", TargetPort: "8000-8010"}, lan)
	if err != nil || v.Name != "Forward 8000-8010" || v.TargetPort != "" || !reflect.DeepEqual(v.Protos, []string{"tcp", "udp"}) {
		t.Fatalf("%+v %v", v, err)
	}
	// Forwarding wan 443 to a lan host is allowed: it shadows nothing (wan input is REJECT).
	if v, err := Validate(ForwardRequest{Name: "web", Proto: "tcp", ExternalPort: "443", TargetIP: "192.168.1.50", TargetPort: "8443"}, lan); err != nil || v.TargetPort != "8443" {
		t.Fatalf("%+v %v", v, err)
	}
	for field, in := range map[string]ForwardRequest{
		"proto":         {Proto: "icmp", ExternalPort: "80", TargetIP: "192.168.1.50"},
		"external_port": {Proto: "tcp", ExternalPort: "0", TargetIP: "192.168.1.50"},
		"target_port":   {Proto: "tcp", ExternalPort: "80-81", TargetIP: "192.168.1.50", TargetPort: "80"},
		"target_ip":     {Proto: "tcp", ExternalPort: "80", TargetIP: "192.168.1.1"},
		"name":          {Name: "a;b", Proto: "tcp", ExternalPort: "80", TargetIP: "192.168.1.50"},
	} {
		var fe *FieldError
		if _, err := Validate(in, lan); !errors.As(err, &fe) || fe.Field != field {
			t.Errorf("%s: %v", field, err)
		}
	}
	for _, ip := range []string{"192.168.1.0", "192.168.1.255", "192.168.2.5", "fd00::5", "10.0.0.1"} {
		if _, err := Validate(ForwardRequest{Proto: "tcp", ExternalPort: "80", TargetIP: ip}, lan); err == nil {
			t.Errorf("%s accepted", ip)
		}
	}
}

func TestAddStagesOwnedRedirect(t *testing.T) {
	s, d, f := setup(t, false)
	st, err := s.Add(t.Context(), ForwardRequest{Name: "NAS", Proto: "both", ExternalPort: "8080", TargetIP: "192.168.1.50", TargetPort: "80"})
	if err != nil || st.State != txn.Pending || f.summary != "Forward tcp/udp 8080 to 192.168.1.50" {
		t.Fatalf("%+v %v %q", st, err, f.summary)
	}
	fw, _ := d.Show(t.Context(), "firewall")
	list := List(fw)
	want := Forward{ID: "travo_fwd_1", Name: "NAS", Proto: "both", ExternalPort: "8080", TargetIP: "192.168.1.50",
		TargetPort: "80", Src: "wan", Dest: "lan", Target: "DNAT", Enabled: true, Owned: true}
	if len(list) != 1 || list[0] != want {
		t.Fatalf("%+v", list)
	}
	// The same external port again overlaps on tcp: refused before anything is staged.
	if _, err := s.Add(t.Context(), ForwardRequest{Proto: "tcp", ExternalPort: "8000-8090", TargetIP: "192.168.1.51"}); !errors.Is(err, ErrPortInUse) {
		t.Fatalf("overlap: %v", err)
	}
	if st, err := s.Add(t.Context(), ForwardRequest{Proto: "udp", ExternalPort: "9000", TargetIP: "192.168.1.51"}); err != nil || st.State != txn.Pending {
		t.Fatalf("second: %v", err)
	}
	if fw, _ := d.Show(t.Context(), "firewall"); List(fw)[1].ID != "travo_fwd_2" {
		t.Fatalf("%+v", List(fw))
	}
}

func TestAddConflictsWithForeignAndRefusesPending(t *testing.T) {
	s, _, f := setup(t, true)
	if _, err := s.Add(t.Context(), ForwardRequest{Proto: "tcp", ExternalPort: "80", TargetIP: "192.168.1.50"}); !errors.Is(err, ErrPortInUse) {
		t.Fatalf("foreign overlap: %v", err)
	}
	// udp does not collide with the tcp-only stock example.
	if _, err := s.Add(t.Context(), ForwardRequest{Proto: "udp", ExternalPort: "80", TargetIP: "192.168.1.50"}); err != nil {
		t.Fatal(err)
	}
	f.active = true
	if _, err := s.Add(t.Context(), ForwardRequest{Proto: "tcp", ExternalPort: "81", TargetIP: "192.168.1.50"}); !errors.Is(err, ErrTransactionPending) {
		t.Fatalf("pending: %v", err)
	}
}

func TestRemoveOwnedOnly(t *testing.T) {
	s, d, _ := setup(t, true)
	if _, err := s.Add(t.Context(), ForwardRequest{Proto: "tcp", ExternalPort: "81", TargetIP: "192.168.1.50"}); err != nil {
		t.Fatal(err)
	}
	fw, _ := d.Show(t.Context(), "firewall")
	foreign := List(fw)[0].ID
	if _, err := s.Remove(t.Context(), foreign); !errors.Is(err, ErrForeign) {
		t.Fatalf("foreign: %v", err)
	}
	if _, err := s.Remove(t.Context(), "travo_fwd_9"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := s.Remove(t.Context(), "travo_fwd_1"); err != nil {
		t.Fatal(err)
	}
	if fw, _ := d.Show(t.Context(), "firewall"); len(List(fw)) != 2 {
		t.Fatalf("%+v", List(fw))
	}
}

func TestStageNeedsZonesAndBoundsOwned(t *testing.T) {
	v := Valid{Name: "x", Protos: []string{"tcp"}, External: PortRange{1, 1}, Target: netip.MustParseAddr("192.168.1.5")}
	noWAN := []device.Section{{Name: "z", Type: "zone", Options: map[string]string{"name": "lan"}}}
	if err := Stage(t.Context(), mock.New(), noWAN, v); !errors.Is(err, ErrNoZone) {
		t.Fatalf("zones: %v", err)
	}
	fw := firewall(t, false)
	for range MaxOwned {
		fw = append(fw, device.Section{Name: freeSection(fw), Type: "redirect", Options: map[string]string{"enabled": "0"}})
	}
	if err := Stage(t.Context(), mock.New(), fw, v); !errors.Is(err, ErrFull) {
		t.Fatalf("full: %v", err)
	}
}
