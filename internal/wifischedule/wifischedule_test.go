package wifischedule

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/crontab"
	"github.com/blackdark-org/travo-greenfield/internal/guard"
	"github.com/blackdark-org/travo-greenfield/internal/hook"
	"github.com/blackdark-org/travo-greenfield/internal/lockout"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
	"github.com/blackdark-org/travo-greenfield/internal/wifi"
)

func rule(id string) Rule {
	return Rule{ID: id, Section: "default_radio1", Days: []int{5, 1, 2}, Off: "22:30", On: "07:05", Enabled: true}
}

func TestValidate(t *testing.T) {
	bad := map[string]func(*Rule){
		"time":       func(r *Rule) { r.Off = "24:00" },
		"equal":      func(r *Rule) { r.On = r.Off },
		"day range":  func(r *Rule) { r.Days = []int{7} },
		"dup day":    func(r *Rule) { r.Days = []int{1, 1} },
		"no days":    func(r *Rule) { r.Days = nil },
		"section":    func(r *Rule) { r.Section = "a;b" },
		"id":         func(r *Rule) { r.ID = "XYZ" },
		"short time": func(r *Rule) { r.On = "7:05" },
	}
	for name, mutate := range bad {
		r := rule("0a0b0c0d")
		mutate(&r)
		if err := (Rules{Rules: []Rule{r}}).Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if err := (Rules{Rules: []Rule{rule("0a0b0c0d"), rule("0a0b0c0d")}}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Error("duplicate ids accepted")
	}
	if err := (Rules{Rules: []Rule{rule(""), rule("")}}).Validate(); err != nil {
		t.Errorf("new rules: %v", err)
	}
}

func TestLinesCallTheHook(t *testing.T) {
	paused := rule("11111111")
	paused.Enabled = false
	got := Lines(Rules{Rules: []Rule{rule("0a0b0c0d"), paused}})
	want := []string{
		"30 22 * * 1,2,5 " + crontab.HookCommand("wifi-schedule-0a0b0c0d-off"),
		"5 7 * * 1,2,5 " + crontab.HookCommand("wifi-schedule-0a0b0c0d-on"),
	}
	if !slices.Equal(got, want) {
		t.Fatalf("lines = %q, want %q", got, want)
	}
	if _, err := crontab.Replace("", Owner, got); err != nil {
		t.Fatalf("shared writer refuses the lines: %v", err)
	}
}

type fakeCron struct {
	lines  []string
	setErr error
	sets   int
}

func (f *fakeCron) Get(context.Context, string) ([]string, error) { return f.lines, nil }
func (f *fakeCron) Set(_ context.Context, owner string, lines []string) error {
	if owner != Owner {
		return errors.New("wrong owner")
	}
	if f.setErr != nil {
		return f.setErr
	}
	if !slices.Equal(f.lines, lines) {
		f.sets++
	}
	f.lines = lines
	return nil
}

type fakeServices struct{ actions []string }

func (f *fakeServices) ServiceRunning(context.Context, string) (bool, error) { return true, nil }
func (f *fakeServices) ServiceAction(_ context.Context, name, action string) error {
	f.actions = append(f.actions, name+" "+action)
	return nil
}

type fakeAPs struct {
	aps    []wifi.APConfig
	got    []wifi.APRequest
	status txn.Status
	err    error
}

func (f *fakeAPs) AccessPoints(context.Context) ([]wifi.APConfig, error) { return f.aps, nil }
func (f *fakeAPs) UpdateAP(_ context.Context, r wifi.APRequest) (txn.Status, error) {
	f.got = append(f.got, r)
	return f.status, f.err
}

type fakeTxns struct{ confirmed []string }

func (f *fakeTxns) Confirm(id string) (txn.Status, error) {
	f.confirmed = append(f.confirmed, id)
	return txn.Status{State: txn.Confirmed}, nil
}

type fakeGuards []guard.Info

func (f fakeGuards) Stale() ([]guard.Info, error) { return f, nil }

func twoAPs() []wifi.APConfig {
	return []wifi.APConfig{
		{Section: "default_radio1", SSID: "Travel", Enabled: true},
		{Section: "guest", SSID: "Guest", Enabled: true, Guest: true},
	}
}

type hooks map[string]hook.Action

func newService(t *testing.T) (*Service, *fakeCron, *fakeAPs, *fakeTxns, hooks) {
	t.Helper()
	cron, aps, txns, registered := &fakeCron{}, &fakeAPs{aps: twoAPs()}, &fakeTxns{}, hooks{}
	return &Service{Path: filepath.Join(t.TempDir(), "wifi-schedule.json"), Cron: cron, Services: &fakeServices{},
		APs: aps, Txns: txns, Guards: fakeGuards(nil),
		Register: func(name string, a hook.Action) {
			if _, dup := registered[name]; dup {
				t.Fatalf("duplicate hook %s", name)
			}
			registered[name] = a
		}}, cron, aps, txns, registered
}

func TestPutLockoutAndIDs(t *testing.T) {
	s, cron, _, _, registered := newService(t)
	ctx := context.Background()
	// Only non-guest AP: refused without acknowledgement, no write.
	if _, err := s.Put(ctx, Rules{Rules: []Rule{rule("")}}); !errors.Is(err, lockout.ErrLockoutRisk) || cron.sets != 0 {
		t.Fatalf("lockout: %v, sets %d", err, cron.sets)
	}
	unknown := rule("")
	unknown.Section = "nope"
	if _, err := s.Put(ctx, Rules{Rules: []Rule{unknown}}); !errors.Is(err, ErrAPNotFound) {
		t.Fatalf("unknown AP: %v", err)
	}
	ack := rule("")
	ack.AcknowledgeLockout = true
	saved, err := s.Put(ctx, Rules{Rules: []Rule{ack}})
	if err != nil || !idPattern.MatchString(saved.Rules[0].ID) {
		t.Fatalf("put: %+v %v", saved, err)
	}
	id := saved.Rules[0].ID
	if len(cron.lines) != 2 || registered[ActionName(id, Off)] == nil || registered[ActionName(id, On)] == nil {
		t.Fatalf("lines %q, hooks %v", cron.lines, registered)
	}
	got, err := s.Get(ctx)
	if err != nil || got.Rules[0].ID != id {
		t.Fatalf("get: %+v %v", got, err)
	}
	if _, err := s.Put(ctx, saved); err != nil || cron.sets != 1 {
		t.Fatalf("unchanged put: %v, sets %d", err, cron.sets)
	}
	// A fresh daemon start registers the stored ids again and re-renders the block.
	again := &Service{Path: s.Path, Cron: &fakeCron{}, Register: func(name string, _ hook.Action) { registered[name+"#2"] = nil }}
	if err := again.Sync(ctx); err != nil || len(again.Cron.(*fakeCron).lines) != 2 {
		t.Fatalf("sync: %v", err)
	}
}

func TestPutRestoresCrontabWhenRuleFileFails(t *testing.T) {
	s, cron, _, _, _ := newService(t)
	cron.lines = []string{"0 1 * * * old"}
	s.writeJSON = func(string, any, os.FileMode) error { return errors.New("no space") }
	r := rule("")
	r.AcknowledgeLockout = true
	if _, err := s.Put(context.Background(), Rules{Rules: []Rule{r}}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v", err)
	}
	if !slices.Equal(cron.lines, []string{"0 1 * * * old"}) {
		t.Fatalf("crontab not restored: %q", cron.lines)
	}
}

func TestTornRuleFileFailsClosed(t *testing.T) {
	s, cron, _, _, _ := newService(t)
	if err := os.WriteFile(s.Path, []byte(`{"rules":[`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatal("torn file read as rules")
	}
	r := rule("")
	r.AcknowledgeLockout = true
	if _, err := s.Put(context.Background(), Rules{Rules: []Rule{r}}); !errors.Is(err, ErrUnavailable) || cron.sets != 0 {
		t.Fatal("torn file overwritten")
	}
	if err := s.Sync(context.Background()); !errors.Is(err, ErrUnavailable) || cron.sets != 0 {
		t.Fatal("sync touched the crontab with an unreadable rule file")
	}
}

// saveAndFire stores r and runs the hook action its cron line would call.
func saveAndFire(t *testing.T, s *Service, registered hooks, r Rule, name string) error {
	t.Helper()
	if _, err := s.Put(context.Background(), Rules{Rules: []Rule{r}}); err != nil {
		t.Fatal(err)
	}
	a := registered[name]
	if a == nil {
		t.Fatalf("no hook %s", name)
	}
	return a(context.Background())
}

func lastOutcome(t *testing.T, s *Service) Outcome {
	t.Helper()
	runs := s.Status(context.Background()).Runs
	if len(runs) != 1 {
		t.Fatalf("runs = %+v", runs)
	}
	return runs[0].Outcome
}

func TestFireAppliesAndConfirms(t *testing.T) {
	s, _, aps, txns, registered := newService(t)
	aps.status = txn.Status{Meta: txn.Meta{ID: "abc"}, State: txn.Pending}
	r := rule("0a0b0c0d")
	r.AcknowledgeLockout = true
	if err := saveAndFire(t, s, registered, r, "wifi-schedule-0a0b0c0d-off"); err != nil {
		t.Fatal(err)
	}
	if got := lastOutcome(t, s); got != Applied {
		t.Fatalf("outcome = %s", got)
	}
	req := aps.got[0]
	if req.Section != "default_radio1" || req.SSID != "Travel" || req.Enabled || !req.Acknowledge || req.Caller.IsValid() || req.Key != "" {
		t.Fatalf("request = %+v", req)
	}
	if len(txns.confirmed) != 1 || txns.confirmed[0] != "abc" {
		t.Fatalf("confirmed = %v", txns.confirmed)
	}
}

func TestFireOutcomes(t *testing.T) {
	cases := map[string]struct {
		setup func(*Service, *fakeAPs)
		hook  string
		want  Outcome
	}{
		"guard held":  {func(s *Service, _ *fakeAPs) { s.Guards = fakeGuards{{Name: "txn"}} }, "off", SkippedGuard},
		"already on":  {func(*Service, *fakeAPs) {}, "on", Unchanged},
		"lockout":     {func(_ *Service, a *fakeAPs) { a.err = lockout.ErrLockoutRisk }, "off", SkippedLockout},
		"pending txn": {func(_ *Service, a *fakeAPs) { a.err = wifi.ErrTransactionPending }, "off", SkippedBusy},
		"probe failed": {func(_ *Service, a *fakeAPs) {
			a.status, a.err = txn.Status{Meta: txn.Meta{ID: "x"}, State: txn.RolledBack}, errors.New("probe")
		}, "off", RolledBack},
		"radio in use": {func(_ *Service, a *fakeAPs) { a.err = wifi.ErrRadioInUse }, "off", Failed},
		"paused rule":  {func(s *Service, _ *fakeAPs) { pauseAll(t, s) }, "off", Unchanged},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			s, _, aps, txns, registered := newService(t)
			r := rule("0a0b0c0d")
			r.AcknowledgeLockout = true
			if _, err := s.Put(context.Background(), Rules{Rules: []Rule{r}}); err != nil {
				t.Fatal(err)
			}
			c.setup(s, aps)
			err := registered["wifi-schedule-0a0b0c0d-"+c.hook](context.Background())
			if got := lastOutcome(t, s); got != c.want {
				t.Fatalf("outcome = %s, want %s", got, c.want)
			}
			if wantErr := c.want != Unchanged; wantErr != (err != nil) || (err != nil && hook.Code(err) != string(c.want)) {
				t.Fatalf("hook error = %v", err)
			}
			if len(txns.confirmed) != 0 {
				t.Fatal("confirmed without a proven transaction")
			}
		})
	}
}

func pauseAll(t *testing.T, s *Service) {
	t.Helper()
	r := rule("0a0b0c0d")
	r.Enabled = false
	if _, err := s.Put(context.Background(), Rules{Rules: []Rule{r}}); err != nil {
		t.Fatal(err)
	}
}
