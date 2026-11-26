package failover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func capture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("../../testdata/container/openwrt-25.12.6/mwan3", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseInterfacesFromCaptures(t *testing.T) {
	for file, want := range map[string]map[string]TrackState{
		"interfaces-online.txt": {
			"wwan": {"online", "active"}, "usbtether": {"online", "active"},
		},
		"interfaces-wwan-offline.txt": {
			"wwan": {"offline", "active"}, "usbtether": {"online", "active"},
		},
		"interfaces-tracking-disabled.txt": {
			"wwan": {"online", "active"}, "usbtether": {"unknown", "disabled"},
		},
	} {
		got := ParseInterfaces(capture(t, file))
		if len(got) != len(want) {
			t.Errorf("%s: %+v", file, got)
		}
		for name, st := range want {
			if got[name] != st {
				t.Errorf("%s: %s = %+v, want %+v", file, name, got[name], st)
			}
		}
	}
}

func TestCheckRuntimeFromCaptures(t *testing.T) {
	uplinks := []string{"wwan", "usbtether"}
	for _, tc := range []struct {
		interfaces, mangle string
		uplinks            []string
		err                string
	}{
		{"interfaces-online.txt", "mangle-online.txt", uplinks, ""},
		// #277: no track_ip → "unknown and tracking is disabled"; the policy never gets a member.
		{"interfaces-tracking-disabled.txt", "mangle-online.txt", uplinks, "tracking of usbtether is disabled"},
		{"interfaces-wwan-offline.txt", "mangle-wwan-offline.txt", uplinks, "wwan offline"},
		// A request through the main table is no proof: the policy must mark for the first uplink.
		{"interfaces-online.txt", "mangle-wwan-offline.txt", uplinks, "does not mark for wwan"},
		{"interfaces-online.txt", "mangle-online.txt", []string{"wwan", "wan"}, "does not report wan"},
	} {
		err := CheckRuntime(capture(t, tc.interfaces), capture(t, tc.mangle), tc.uplinks)
		if tc.err == "" && err != nil || tc.err != "" && (err == nil || !strings.Contains(err.Error(), tc.err)) {
			t.Errorf("%s + %s: %v, want %q", tc.interfaces, tc.mangle, err, tc.err)
		}
	}
}

func TestCheckRuntimeNeedsHooksAndTravoRuleFirst(t *testing.T) {
	interfaces, mangle := capture(t, "interfaces-online.txt"), capture(t, "mangle-online.txt")
	for name, edit := range map[string]func(string) string{
		"no OUTPUT hook": func(s string) string { return strings.Replace(s, "-A OUTPUT -j mwan3_hook\n", "", 1) },
		"stock rule first": func(s string) string {
			return strings.Replace(s, "-A mwan3_rules ", "-A mwan3_rules -p tcp -m multiport --dports 443 -j mwan3_rule_https\n-A mwan3_rules ", 1)
		},
	} {
		if err := CheckRuntime(interfaces, edit(mangle), []string{"wwan", "usbtether"}); err == nil {
			t.Errorf("%s: passed", name)
		}
	}
}
