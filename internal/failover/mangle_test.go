package failover

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/pkgmgr"
)

func TestPlanMangleCleanupTakesOnlyMWAN3(t *testing.T) {
	left, err := PlanMangleCleanup("ip", capture(t, "nft-ruleset-ip-leftover.json"))
	if err != nil {
		t.Fatal(err)
	}
	wantRules := []device.NftRule{{Chain: "PREROUTING", Handle: 21}, {Chain: "OUTPUT", Handle: 23}, {Chain: "PREROUTING", Handle: 43}, {Chain: "OUTPUT", Handle: 44}}
	slices.SortFunc(left.Rules, func(a, b device.NftRule) int { return a.Handle - b.Handle })
	if !slices.Equal(left.Rules, wantRules) {
		t.Fatalf("rules = %+v", left.Rules)
	}
	if len(left.Chains) != 15 || slices.Contains(left.Chains, "ts-probe") || slices.Contains(left.Chains, "PREROUTING") ||
		!slices.Contains(left.Chains, "mwan3_policy_travo_failover") || !slices.Contains(left.Chains, "mwan3_hook") {
		t.Fatalf("chains = %v", left.Chains)
	}
}

func TestPlanMangleCleanupIsEmptyWhenClean(t *testing.T) {
	for family, name := range map[string]string{"ip": "nft-ruleset-ip-cleaned.json", "ip6": "nft-ruleset-ip6-cleaned.json", "arp": "nft-ruleset-empty.json"} {
		if left, err := PlanMangleCleanup(family, capture(t, name)); err != nil || !left.Empty() {
			t.Fatalf("%s: %+v, %v", family, left, err)
		}
	}
	if _, err := PlanMangleCleanup("ip", "iptables v1.8.10"); err == nil {
		t.Fatal("garbage parsed")
	}
}

func TestPlanMangleCleanupRefusesForeignJumps(t *testing.T) {
	// The ts-probe chain of the capture, jumping into mwan3_hook instead of marking.
	raw := strings.Replace(capture(t, "nft-ruleset-ip-leftover.json"),
		`"chain": "ts-probe", "handle": 69, "expr": [`, `"chain": "ts-probe", "handle": 69, "expr": [{"jump": {"target": "mwan3_hook"}}, `, 1)
	if _, err := PlanMangleCleanup("ip", raw); !errors.Is(err, ErrForeignJump) {
		t.Fatalf("err = %v", err)
	}
}

func leftoverRig(t *testing.T) *rig {
	t.Helper()
	r := newRig(t, true)
	r.d.NftRulesets = map[string]string{
		"ip":  capture(t, "nft-ruleset-ip-leftover.json"),
		"ip6": capture(t, "nft-ruleset-ip6-leftover.json"),
	}
	return r
}

func TestDisableRemovesMWAN3LeftoversAndKeepsForeignRules(t *testing.T) {
	r := leftoverRig(t)
	if err := r.s.Disable(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, family := range MangleFamilies {
		raw, _ := r.d.NftRuleset(context.Background(), family)
		if strings.Contains(raw, "mwan3_") || !strings.Contains(raw, `"target":"ts-probe"`) || !strings.Contains(raw, `"name":"ts-probe"`) {
			t.Fatalf("%s after disable: %s", family, raw)
		}
	}
	if r.d.MangleDeletes != 2 {
		t.Fatalf("deletes = %d", r.d.MangleDeletes)
	}
	if err := r.s.Disable(context.Background()); err != nil || r.d.MangleDeletes != 2 {
		t.Fatalf("second disable: %v, deletes %d", err, r.d.MangleDeletes)
	}
}

func TestDisableCleansWithoutTheMWAN3Package(t *testing.T) {
	r := leftoverRig(t)
	_ = r.pm.Remove(context.Background(), Package)
	if err := r.s.Disable(context.Background()); err != nil || r.d.MangleDeletes != 2 {
		t.Fatalf("disable: %v, deletes %d", err, r.d.MangleDeletes)
	}
}

func TestDisableReportsAFailedCleanup(t *testing.T) {
	r := leftoverRig(t)
	r.d.DeleteMangleErr = errors.New("nft: Error: Could not process rule")
	if err := r.s.Disable(context.Background()); err == nil {
		t.Fatal("want error")
	}
	st, err := r.s.Status(context.Background())
	if err != nil || st.Enabled || st.Blocked {
		t.Fatalf("status = %+v, %v (mwan3 stopped, the guard released; disabling again retries)", st, err)
	}
}

func TestEnableRefusesWhileTailscaleRuns(t *testing.T) {
	r := newRig(t, true)
	_ = r.pm.Install(context.Background(), TailscalePackage)
	r.d.SetService(TailscalePackage, true, true)
	if _, err := r.s.Enable(context.Background(), []string{"wan", "wwan"}); !errors.Is(err, ErrTailscale) {
		t.Fatalf("err = %v", err)
	}
	if on, _ := r.s.Enabled(); on || len(r.d.ServiceActions) != 0 {
		t.Fatalf("refusal changed state: enabled %v, actions %v", on, r.d.ServiceActions)
	}
	r.d.SetService(TailscalePackage, true, false) // installed, stopped
	if _, err := r.s.Enable(context.Background(), []string{"wan", "wwan"}); err != nil {
		t.Fatal(err)
	}
	if on, err := r.s.Enabled(); !on || err != nil {
		t.Fatalf("enabled = %v, %v", on, err)
	}
}

// failingQuery fails the version query of one package.
type failingQuery struct {
	*pkgmgr.Mock
	pkg string
}

func (f failingQuery) Version(ctx context.Context, pkg string) (string, error) {
	if pkg == f.pkg {
		return "", errors.New("apk: database locked")
	}
	return f.Mock.Version(ctx, pkg)
}

func TestEnableTreatsAFailedTailscaleQueryAsAnError(t *testing.T) {
	r := newRig(t, true)
	r.s.Packages = failingQuery{Mock: r.pm, pkg: TailscalePackage}
	if _, err := r.s.Enable(context.Background(), []string{"wan", "wwan"}); err == nil || errors.Is(err, ErrTailscale) {
		t.Fatalf("err = %v", err)
	}
	if on, _ := r.s.Enabled(); on {
		t.Fatal("enabled past a failed check")
	}
}
