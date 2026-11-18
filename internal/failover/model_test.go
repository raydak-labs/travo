package failover

import (
	"context"
	"errors"
	"maps"
	"os"
	"slices"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
)

// stockRouter loads the mwan3 package's own /etc/config/mwan3 (25.12.6 feed, verbatim);
// `uci export` adds only the package line the parser expects.
func stockRouter(t *testing.T) *mock.Device {
	t.Helper()
	b, err := os.ReadFile("../../testdata/device/axt1800-25.12.6/mwan3/stock-config-mwan3.txt")
	if err != nil {
		t.Fatal(err)
	}
	secs, err := cli.ParseExport(Config, "package mwan3\n\n"+string(b))
	if err != nil {
		t.Fatal(err)
	}
	d := mock.New()
	if err := d.Load(context.Background(), Config, secs); err != nil {
		t.Fatal(err)
	}
	return d
}

func show(t *testing.T, d *mock.Device) []device.Section {
	t.Helper()
	secs, err := d.Show(context.Background(), Config)
	if err != nil {
		t.Fatal(err)
	}
	return secs
}

func get(secs []device.Section, name string) (device.Section, bool) {
	i := slices.IndexFunc(secs, func(s device.Section) bool { return s.Name == name })
	if i < 0 {
		return device.Section{}, false
	}
	return secs[i], true
}

// apply plans, stages and commits like the transaction does.
func apply(t *testing.T, d *mock.Device, s Settings) Plan {
	t.Helper()
	ctx := context.Background()
	before := show(t, d)
	p, err := PlanApply(before, s)
	if err != nil {
		t.Fatal(err)
	}
	if err := Stage(ctx, d, d, before, p, false); err != nil {
		t.Fatal(err)
	}
	if err := d.Commit(ctx, Config); err != nil {
		t.Fatal(err)
	}
	if err := Verify(show(t, d), p, false); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestValidate(t *testing.T) {
	for _, s := range []Settings{
		{Enabled: true, Uplinks: []string{"wan"}},
		{Enabled: true, Uplinks: []string{"wan", "wan"}},
		{Enabled: true, Uplinks: []string{"wan", "wg0"}},
		{Uplinks: []string{"lan"}},
	} {
		if err := s.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v: %v, want ErrInvalid", s, err)
		}
	}
	for _, s := range []Settings{{Enabled: true, Uplinks: []string{"wwan", "wan"}}, {Uplinks: []string{}}, {Enabled: true, Uplinks: Candidates}} {
		if err := s.Validate(); err != nil {
			t.Errorf("%+v: %v", s, err)
		}
	}
}

func TestApplyOnStockConfigAdoptsWanAndKeepsStockSections(t *testing.T) {
	d := stockRouter(t)
	stock := show(t, d)
	p := apply(t, d, Settings{Enabled: true, Uplinks: []string{"wan", "wwan"}})
	if !slices.Equal(p.Warnings, []Warning{{Section: "wan", Reason: "adopted"}}) {
		t.Fatalf("warnings = %+v", p.Warnings)
	}
	if len(p.Delete) != 0 || len(p.Disable) != 0 {
		t.Fatalf("plan touches stock sections: %+v", p)
	}
	after := show(t, d)
	for _, s := range stock {
		got, ok := get(after, s.Name)
		if !ok {
			t.Fatalf("stock section %s deleted", s.Name)
		}
		if s.Name != "wan" && (!maps.Equal(got.Options, s.Options) || !maps.EqualFunc(got.Lists, s.Lists, slices.Equal)) {
			t.Errorf("stock section %s edited: %+v", s.Name, got)
		}
	}
	wan, _ := get(after, "wan")
	if wan.Options["enabled"] != "1" || wan.Options["family"] != "ipv4" || len(wan.Lists["track_ip"]) != 0 || wan.Options["reliability"] != "2" {
		t.Fatalf("adopted wan = %+v (track_ip must go, reliability stays)", wan)
	}
	m2, _ := get(after, MemberName("wwan"))
	pol, _ := get(after, Policy)
	v4, _ := get(after, RuleV4)
	v6, _ := get(after, RuleV6)
	if m2.Options["metric"] != "2" || !slices.Equal(pol.Lists["use_member"], []string{"travo_m_wan", "travo_m_wwan"}) ||
		pol.Options["last_resort"] != "default" || v4.Options["use_policy"] != Policy || v6.Options["use_policy"] != "default" {
		t.Fatalf("generated sections: %+v %+v %+v %+v", m2, pol, v4, v6)
	}
	if !slices.Equal(d.Reorders, []string{"mwan3.travo_v4=0", "mwan3.travo_v6=1"}) {
		t.Fatalf("reorders = %v", d.Reorders)
	}
	// Re-applying the same settings stages nothing and warns about nothing.
	again, err := PlanApply(after, Settings{Enabled: true, Uplinks: []string{"wan", "wwan"}})
	if err != nil || len(again.Warnings) != 0 {
		t.Fatalf("re-plan = %+v, %v", again, err)
	}
	if err := Stage(context.Background(), d, d, after, again, true); err != nil {
		t.Fatal(err)
	}
	if pending, _ := d.Pending(context.Background(), Config); pending {
		t.Fatal("unchanged re-apply staged changes")
	}
}

func TestOperatorEditedSectionIsOverwrittenLoudly(t *testing.T) {
	d := stockRouter(t)
	s := Settings{Enabled: true, Uplinks: []string{"wwan", "wan"}}
	apply(t, d, s)
	ctx := context.Background()
	if err := d.Set(ctx, Config, MemberName("wwan"), "metric", "9"); err != nil {
		t.Fatal(err)
	}
	if err := d.Commit(ctx, Config); err != nil {
		t.Fatal(err)
	}
	p := apply(t, d, s)
	if !slices.Contains(p.Warnings, Warning{Section: MemberName("wwan"), Reason: "operator_edited"}) {
		t.Fatalf("warnings = %+v", p.Warnings)
	}
	if m, _ := get(show(t, d), MemberName("wwan")); m.Options["metric"] != "1" {
		t.Fatalf("metric not restored: %+v", m)
	}
}

func TestRemovingAnUplinkDeletesOnlyStampedSections(t *testing.T) {
	d := stockRouter(t)
	apply(t, d, Settings{Enabled: true, Uplinks: Candidates})
	ctx := context.Background()
	// Hand-edit the wwan member: it is no longer Travo's to delete.
	if err := d.Set(ctx, Config, MemberName("wwan"), "weight", "5"); err != nil {
		t.Fatal(err)
	}
	if err := d.Commit(ctx, Config); err != nil {
		t.Fatal(err)
	}
	p := apply(t, d, Settings{Enabled: true, Uplinks: []string{"wan", "usbtether"}})
	if !slices.Equal(p.Disable, []string{"wwan"}) || slices.Contains(p.Delete, MemberName("wwan")) {
		t.Fatalf("plan = %+v", p)
	}
	if !slices.Contains(p.Warnings, Warning{Section: MemberName("wwan"), Reason: "stale_edited"}) {
		t.Fatalf("warnings = %+v", p.Warnings)
	}
	after := show(t, d)
	if w, ok := get(after, "wwan"); !ok || w.Options["enabled"] != "0" || !Stamped(w) {
		t.Fatalf("wwan interface = %+v, %v (kept, disabled, stamped)", w, ok)
	}
	if _, ok := get(after, MemberName("wwan")); !ok {
		t.Fatal("hand-edited member deleted")
	}
	// The stock interface sections Travo never stamped are never disabled or deleted.
	if wanb, ok := get(after, "wanb"); !ok || wanb.Options["enabled"] != "0" || wanb.Options[FingerprintOption] != "" {
		t.Fatalf("stock wanb = %+v", wanb)
	}
}

func TestForeignSectionOnOwnedNameConflicts(t *testing.T) {
	d := stockRouter(t)
	ctx := context.Background()
	if err := d.SetType(ctx, Config, Policy, "rule"); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanApply(show(t, d), Settings{Enabled: true, Uplinks: []string{"wan", "wwan"}}); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v", err)
	}
}

func TestVerifyCatchesMissingAndUnorderedRules(t *testing.T) {
	p, _ := PlanApply(nil, Settings{Enabled: true, Uplinks: []string{"wan", "wwan"}})
	if err := Verify(nil, p, false); err == nil {
		t.Fatal("empty config verified")
	}
	ordered := append([]device.Section{{Name: "https", Type: "rule"}}, p.Write...)
	if err := Verify(ordered, p, true); err == nil {
		t.Fatal("stock rule before the Travo rules verified")
	}
	if err := Verify(p.Write, p, true); err != nil {
		t.Fatal(err)
	}
}

func TestFingerprintIgnoresUnownedOptions(t *testing.T) {
	s := Desired(Settings{Enabled: true, Uplinks: []string{"wan", "wwan"}})[0]
	s.Options["reliability"] = "2"
	if !Stamped(s) {
		t.Fatal("an unowned option broke the stamp")
	}
	s.Lists["track_ip"] = []string{"1.1.1.1"}
	if Stamped(s) {
		t.Fatal("an added track_ip kept the stamp")
	}
}
