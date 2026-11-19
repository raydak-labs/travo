package leds

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/crontab"
	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/cli"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
)

const capture = "../../testdata/device/axt1800-25.12.6/leds/"

func readCapture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(capture + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func sysfsCapture(t *testing.T, name string) []device.LED {
	t.Helper()
	l, err := mock.ParseLEDsTSV([]byte(readCapture(t, name)))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestEntriesFromCapture(t *testing.T) {
	got := Entries(sysfsCapture(t, "sysfs-default.tsv"))
	want := []Entry{{Name: "blue:run", Trigger: "none", On: true}, {Name: "white:system", Trigger: "none"}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v", got)
	}
	for _, e := range Entries(sysfsCapture(t, "sysfs-off.tsv")) {
		if e.On {
			t.Errorf("%s on after the off probe", e.Name)
		}
	}
	if ActiveTrigger("none timer [mmc0] phy0rx") != "mmc0" || ActiveTrigger("none timer") != "" {
		t.Fatal("active trigger")
	}
	if e := Entries([]device.LED{{Name: "x", Trigger: "none [heartbeat]"}}); !e[0].On {
		t.Fatal("a triggered LED with brightness 0 is on")
	}
}

// The section Travo writes is the one the probe committed and `led restart` applied.
func TestOffSectionMatchesCapture(t *testing.T) {
	var lines []string
	for l := range strings.SplitSeq(readCapture(t, "uci-show-system-off-grep-led.txt"), "\n") {
		if strings.HasPrefix(l, "system."+SectionPrefix) {
			lines = append(lines, l)
		}
	}
	secs, err := cli.ParseShow("system", strings.Join(lines, "\n")+"\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(secs) != 2 {
		t.Fatalf("got %+v", secs)
	}
	for _, s := range secs {
		want := OffSection(s.Options["sysfs"])
		if s.Name != want.Name || s.Type != want.Type || !sameOptions(s, want) || len(s.Options) != len(want.Options) {
			t.Errorf("captured %+v, Travo writes %+v", s, want)
		}
	}
	if SectionName("a-b:c.d") != "travo_led_a_b_c_d" {
		t.Fatal(SectionName("a-b:c.d"))
	}
}

func TestPlansTouchOnlyTravoSections(t *testing.T) {
	sysfs := sysfsCapture(t, "sysfs-default.tsv")
	foreign := device.Section{Name: "led_wan", Type: "led", Options: map[string]string{"sysfs": "blue:run", "trigger": "netdev"}}
	stale := OffSection("gone:led")
	current := OffSection("blue:run")
	current.Options["delayon"] = "1" // an operator's extra option is kept
	secs := []device.Section{foreign, stale, current}
	p := PlanOff(secs, sysfs)
	if len(p.Write) != 1 || p.Write[0].Name != "travo_led_white_system" || !slices.Equal(p.Delete, []string{stale.Name}) {
		t.Fatalf("off plan %+v", p)
	}
	d := PlanDefault(secs)
	if len(d.Write) != 0 || !slices.Equal(d.Delete, []string{stale.Name, current.Name}) {
		t.Fatalf("default plan %+v", d)
	}
	if !PlanDefault([]device.Section{foreign}).Empty() {
		t.Fatal("no Travo section: nothing to do")
	}
}

type commitApply struct{ dev *mock.Device }

func (a commitApply) Apply(ctx context.Context, configs []string, stage func(context.Context) error) error {
	if err := stage(ctx); err != nil {
		return err
	}
	for _, c := range configs {
		if err := a.dev.Commit(ctx, c); err != nil {
			return err
		}
	}
	return nil
}

func newService(t *testing.T) (*Service, *mock.Device) {
	t.Helper()
	dev := mock.New()
	secs, err := cli.ParseShow("system", readCapture(t, "uci-show-system-default.txt"))
	if err != nil {
		t.Fatal(err)
	}
	secs = append(secs, device.Section{Name: "led_wan", Type: "led", Options: map[string]string{"sysfs": "white:system", "trigger": "netdev"}})
	if err := dev.Load(context.Background(), "system", secs); err != nil {
		t.Fatal(err)
	}
	dev.SetService("led", true, false)
	dev.SetService("cron", true, false)
	apply := commitApply{dev}
	cron := &crontab.Service{File: dev, UCI: dev, Apply: apply, Services: dev}
	return &Service{UCI: dev, Sysfs: dev, Apply: apply, Services: dev, Cron: cron}, dev
}

func TestSetOffAndBackToDefault(t *testing.T) {
	ctx := context.Background()
	s, dev := newService(t)
	st, err := s.SetOff(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Off || slices.ContainsFunc(st.LEDs, func(e Entry) bool { return e.On }) {
		t.Fatalf("after off %+v", st)
	}
	if _, err := s.SetOff(ctx, true); err != nil {
		t.Fatal(err)
	}
	if n := slices.Index(dev.ServiceActions, "led restart"); n < 0 || len(dev.ServiceActions) != 1 {
		t.Fatalf("restarts %q: an unchanged config must not restart", dev.ServiceActions)
	}
	st, err = s.SetOff(ctx, false)
	if err != nil || st.Off || !st.LEDs[0].On {
		t.Fatalf("after default %+v %v", st, err)
	}
	secs, _ := dev.Show(ctx, "system")
	if len(Owned(secs)) != 0 || !slices.ContainsFunc(secs, func(s device.Section) bool { return s.Name == "led_wan" }) {
		t.Fatalf("sections %+v", secs)
	}
}

func TestScheduleKeepsForeignCronLines(t *testing.T) {
	ctx := context.Background()
	s, dev := newService(t)
	const foreign = "*/5 * * * * /usr/bin/check.sh\n"
	_ = dev.WriteCrontab(ctx, foreign)
	st, err := s.SetSchedule(ctx, Schedule{Enabled: true, OffAt: "22:30", OnAt: "06:45"})
	if err != nil || st.Schedule != (Schedule{Enabled: true, OffAt: "22:30", OnAt: "06:45"}) {
		t.Fatalf("%+v %v", st.Schedule, err)
	}
	got, _ := dev.ReadCrontab(ctx)
	want := foreign + "# travo-greenfield:leds begin\n" +
		"30 22 * * * /usr/bin/travo-greenfield --hook leds-off 2>&1 | logger -t travo-greenfield-hook\n" +
		"45 6 * * * /usr/bin/travo-greenfield --hook leds-on 2>&1 | logger -t travo-greenfield-hook\n" +
		"# travo-greenfield:leds end\n"
	if got != want {
		t.Fatalf("crontab %q", got)
	}
	st, err = s.SetSchedule(ctx, Schedule{OffAt: "22:30", OnAt: "06:45"})
	if err != nil || st.Schedule != (Schedule{OffAt: DefaultOffAt, OnAt: DefaultOnAt}) {
		t.Fatalf("%+v %v", st.Schedule, err)
	}
	if got, _ := dev.ReadCrontab(ctx); got != foreign {
		t.Fatalf("crontab %q", got)
	}
	for _, bad := range []Schedule{{Enabled: true, OffAt: "07:00", OnAt: "07:00"}, {OffAt: "7:00", OnAt: "08:00"}} {
		if _, err := s.SetSchedule(ctx, bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v: %v", bad, err)
		}
	}
}

func TestParseScheduleIgnoresEditedBlock(t *testing.T) {
	edited := []string{"30 22 * * 1 " + crontab.HookCommand(ActionOff), "0 7 * * * " + crontab.HookCommand(ActionOn)}
	if got := ParseSchedule(edited); got.Enabled {
		t.Fatalf("%+v", got)
	}
}

func TestHookActions(t *testing.T) {
	ctx := context.Background()
	s, _ := newService(t)
	a := s.Actions()
	if err := a[ActionOff](ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ := s.Get(ctx); !st.Off {
		t.Fatal("leds-off did not hold the LEDs off")
	}
	if err := a[ActionOn](ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ := s.Get(ctx); st.Off {
		t.Fatal("leds-on did not return the LEDs to default")
	}
}
