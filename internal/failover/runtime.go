package failover

import (
	"fmt"
	"regexp"
	"strings"
)

// TrackState is one line of `mwan3 interfaces`
// (testdata/container/openwrt-25.12.6/mwan3/interfaces-*.txt).
type TrackState struct {
	// Status is mwan3track's verdict: online, offline, connecting, disconnecting, unknown.
	Status string
	// Tracking is active, paused, down, disabled or "not enabled".
	Tracking string
}

var interfaceLine = regexp.MustCompile(`^ interface (\S+) is (\S+) and tracking is (.+?)(?: \(.*\))?$`)

// ParseInterfaces reads `mwan3 interfaces` into a state per mwan3 interface section.
func ParseInterfaces(out string) map[string]TrackState {
	states := map[string]TrackState{}
	for line := range strings.SplitSeq(out, "\n") {
		if m := interfaceLine.FindStringSubmatch(line); m != nil {
			states[m[1]] = TrackState{Status: m[2], Tracking: m[3]}
		}
	}
	return states
}

// CheckRuntime proves mwan3 routes by Travo's policy, not merely that a request left
// through the main table (#277): every uplink is tracked and online, and the IPv4 mangle
// rules (`iptables -t mangle -S`) hook PREROUTING and OUTPUT, send unmarked traffic to
// travo_failover first and mark it for the first uplink, the lowest metric.
func CheckRuntime(interfaces, mangle string, uplinks []string) error {
	states := ParseInterfaces(interfaces)
	for _, u := range uplinks {
		st, ok := states[u]
		switch {
		case !ok:
			return fmt.Errorf("mwan3 does not report %s", u)
		case st.Tracking != "active":
			return fmt.Errorf("mwan3 tracking of %s is %s, not active", u, st.Tracking)
		case st.Status != "online":
			return fmt.Errorf("mwan3 reports %s %s, not online", u, st.Status)
		}
	}
	rules := strings.Split(mangle, "\n")
	for _, hook := range []string{"-A PREROUTING -j mwan3_hook", "-A OUTPUT -j mwan3_hook"} {
		if !hasLine(rules, hook) {
			return fmt.Errorf("mwan3 mangle hook %q missing", hook)
		}
	}
	chain := "mwan3_policy_" + Policy
	if first := firstRule(rules, "mwan3_rules"); !strings.HasSuffix(first, " -j "+chain) {
		return fmt.Errorf("mwan3's first rule is %q, not %s", first, chain)
	}
	want := fmt.Sprintf(`--comment "%s 1 1"`, uplinks[0])
	if member := firstRule(rules, chain); !strings.Contains(member, want) {
		return fmt.Errorf("policy %s does not mark for %s: %q", Policy, uplinks[0], member)
	}
	return nil
}

func hasLine(lines []string, want string) bool {
	for _, l := range lines {
		if strings.TrimSpace(l) == want {
			return true
		}
	}
	return false
}

func firstRule(lines []string, chain string) string {
	prefix := "-A " + chain + " "
	for _, l := range lines {
		if strings.HasPrefix(l, prefix) {
			return strings.TrimSpace(l)
		}
	}
	return ""
}
