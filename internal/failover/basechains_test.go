package failover

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
	"github.com/blackdark-org/travo-greenfield/internal/txn"
)

// deviceCapture reads an AXT1800 capture (testdata/device/axt1800-25.12.6/mwan3/PROVENANCE).
func deviceCapture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("../../testdata/device/axt1800-25.12.6/mwan3", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// #277 on the AXT1800: a stopped Tailscale left `ip`/`ip6 mangle` OUTPUT as `type filter`;
// iptables-nft wants `type route`, so mwan3's setup batch failed and mwan3_hook never existed.
// PREROUTING matches iptables-nft and stays.
func TestPlanStaleBaseChainsFromDeviceCapture(t *testing.T) {
	for _, family := range MangleFamilies {
		got, err := PlanStaleBaseChains(family, deviceCapture(t, "nft-ruleset-"+family+"-stale-base.json"))
		if err != nil || !slices.Equal(got, []string{"OUTPUT"}) {
			t.Errorf("%s: %v, %v", family, got, err)
		}
	}
	for _, name := range []string{"nft-ruleset-ip-leftover.json", "nft-ruleset-ip-cleaned.json", "nft-ruleset-empty.json"} {
		if got, err := PlanStaleBaseChains("ip", capture(t, name)); err != nil || len(got) != 0 {
			t.Errorf("%s (iptables-nft's own chains): %v, %v", name, got, err)
		}
	}
}

func TestPlanStaleBaseChainsRefusesChainsThatFilter(t *testing.T) {
	raw := deviceCapture(t, "nft-ruleset-ip-stale-base.json")
	for name, edit := range map[string]string{
		"drop policy":  strings.Replace(raw, `"hook": "output", "prio": -150, "policy": "accept"`, `"hook": "output", "prio": -150, "policy": "drop"`, 1),
		"holds a rule": strings.Replace(raw, `]}`, `, {"rule": {"family": "ip", "table": "mangle", "chain": "OUTPUT", "handle": 4, "expr": []}}]}`, 1),
	} {
		if _, err := PlanStaleBaseChains("ip", edit); !errors.Is(err, ErrMangleIncompatible) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestEnableDeletesStaleBaseChainsBeforeMwan3Starts(t *testing.T) {
	r := newRig(t, true)
	r.d.NftRulesets = map[string]string{
		"ip":  deviceCapture(t, "nft-ruleset-ip-stale-base.json"),
		"ip6": deviceCapture(t, "nft-ruleset-ip6-stale-base.json"),
	}
	st, err := r.s.Enable(context.Background(), []string{"wan", "wwan"})
	if err != nil || st.State != txn.Pending {
		t.Fatalf("enable = %+v, %v (probe %v)", st, err, r.txns.probeErr)
	}
	for _, family := range MangleFamilies {
		if left, err := PlanStaleBaseChains(family, r.d.NftRulesets[family]); err != nil || len(left) != 0 {
			t.Errorf("%s: %v left, %v", family, left, err)
		}
		if !strings.Contains(r.d.NftRulesets[family], `"PREROUTING"`) {
			t.Errorf("%s: compatible PREROUTING deleted", family)
		}
	}
}

func TestEnableRefusesAnIncompatibleMangleTable(t *testing.T) {
	r := newRig(t, true)
	r.d.MangleErr = errors.New("iptables v1.8.10 (nf_tables): table `mangle' is incompatible, use 'nft' tool.")
	if _, err := r.s.Enable(context.Background(), []string{"wan", "wwan"}); !errors.Is(err, ErrMangleIncompatible) {
		t.Fatalf("enable: %v", err)
	}
	if len(r.d.ServiceActions) != 0 || r.txns.probeErr != nil {
		t.Fatalf("refusal acted: %v", r.d.ServiceActions)
	}
	r.d.MangleErr = nil
	r.d.MangleOut = deviceCapture(t, "mangle-stale-base.txt") // read cleanly, but incompatible
	if _, err := r.s.Enable(context.Background(), []string{"wan", "wwan"}); !errors.Is(err, ErrMangleIncompatible) {
		t.Fatalf("enable with incompatible base chains: %v", err)
	}
}

// racingMangle adds a rule to the stale chain right after Enable read it as empty.
type racingMangle struct {
	*mock.Device
	reads int
}

func (m *racingMangle) NftRuleset(ctx context.Context, family string) (string, error) {
	raw, err := m.Device.NftRuleset(ctx, family)
	if m.reads++; m.reads == 1 {
		m.Device.NftRulesets[family] = strings.Replace(raw, `]}`, `, {"rule": {"family": "ip", "table": "mangle", "chain": "OUTPUT", "handle": 9, "expr": []}}]}`, 1)
	}
	return raw, err
}

// #283 review: a chain that gained a rule after the check is never flushed; Enable refuses.
func TestEnableNeverFlushesAChainThatGainedARule(t *testing.T) {
	r := newRig(t, true)
	r.d.NftRulesets = map[string]string{"ip": deviceCapture(t, "nft-ruleset-ip-stale-base.json")}
	r.s.Mangle = &racingMangle{Device: r.d}
	if _, err := r.s.Enable(context.Background(), []string{"wan", "wwan"}); !errors.Is(err, ErrMangleIncompatible) {
		t.Fatalf("enable: %v", err)
	}
	if !strings.Contains(r.d.NftRulesets["ip"], `"handle": 9`) || len(r.d.ServiceActions) != 0 {
		t.Fatalf("chain emptied or mwan3 started: %v", r.d.ServiceActions)
	}
}

// The probe of the failed live apply: both uplinks tracked online (the parenthesis says
// "offline" for the last change), but no mwan3_hook and only "out" rules in the policy.
func TestCheckRuntimeFromFailedDeviceProbe(t *testing.T) {
	interfaces := deviceCapture(t, "interfaces-wan-usbtether-online.txt")
	st := ParseInterfaces(interfaces)
	for _, u := range []string{"wan", "usbtether"} {
		if st[u] != (TrackState{"online", "active"}) {
			t.Errorf("%s = %+v", u, st[u])
		}
	}
	err := CheckRuntime(interfaces, deviceCapture(t, "mangle-probe-no-hook.txt"), []string{"wan", "usbtether"})
	var pe *ProbeError
	if !errors.As(err, &pe) || pe.Check != CheckHook {
		t.Fatalf("CheckRuntime = %v, want check hook", err)
	}
}
