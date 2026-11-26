package tailscale

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const fixtures = "../../testdata/device/axt1800-25.12.6/tailscale"

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fixtures, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseNeedsLoginAfterInstall(t *testing.T) {
	b, err := Parse(fixture(t, "status-needslogin.json"), fixture(t, "prefs-default.json"))
	if err != nil {
		t.Fatal(err)
	}
	if b.State != StateNeedsLogin || b.AuthURL != "" {
		t.Fatalf("state %q auth %q", b.State, b.AuthURL)
	}
	if b.Self.HostName != "OpenWrt" || b.Self.IPs == nil || len(b.Self.IPs) != 0 {
		t.Fatalf("self %+v", b.Self)
	}
	if b.ExitNodes == nil || len(b.ExitNodes) != 0 || b.MagicDNS.Enabled {
		t.Fatalf("exit nodes %+v magic %+v", b.ExitNodes, b.MagicDNS)
	}
	// The package default: tailscaled would take over DNS until Travo pins it off.
	if !b.Prefs.AcceptDNS || b.Prefs.WantRunning || b.Prefs.AdvertiseRoutes == nil {
		t.Fatalf("prefs %+v", b.Prefs)
	}
	if !slices.Equal(b.Health, []string{"Tailscale is stopped."}) {
		t.Fatalf("health %v", b.Health)
	}
}

func TestParseKeepsAuthURLAfterUpTimeout(t *testing.T) {
	b, err := Parse(fixture(t, "status-authurl.json"), fixture(t, "prefs-after-up.json"))
	if err != nil {
		t.Fatal(err)
	}
	if b.State != StateNeedsLogin || !strings.HasPrefix(b.AuthURL, "https://login.tailscale.com/a/") {
		t.Fatalf("state %q auth %q", b.State, b.AuthURL)
	}
	if b.Prefs.AcceptDNS || !b.Prefs.WantRunning {
		t.Fatalf("prefs %+v", b.Prefs)
	}
}

func TestParseUnresolvedExitNodeKeepsIP(t *testing.T) {
	b, err := Parse(fixture(t, "status-authurl.json"), fixture(t, "prefs-ssh-routes.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := Prefs{ExitNode: "100.64.0.1", AdvertiseRoutes: []string{"192.168.1.0/24"}, SSH: true, WantRunning: true}
	if b.Prefs.ExitNode != want.ExitNode || !slices.Equal(b.Prefs.AdvertiseRoutes, want.AdvertiseRoutes) || !b.Prefs.SSH {
		t.Fatalf("prefs %+v", b.Prefs)
	}
	if d := CurrentDesired(b); d.ExitNodeIP != "100.64.0.1" || !d.SSH {
		t.Fatalf("desired %+v", d)
	}
}

// runningJSON is a Running status assembled only from field names present in the captures
// (Peer entries share Self's PeerStatus shape); the verbatim capture needs a real login.
const runningJSON = `{"Version":"1.104.1-1 (OpenWrt)","BackendState":"Running","AuthURL":"",
"Self":{"ID":"nSelf","HostName":"OpenWrt","DNSName":"openwrt.tail0000.ts.net.","TailscaleIPs":["100.64.0.1","fd7a:115c:a1e0::1"],"Online":true},
"Health":[],"MagicDNSSuffix":"tail0000.ts.net",
"CurrentTailnet":{"Name":"example.com","MagicDNSSuffix":"tail0000.ts.net","MagicDNSEnabled":true},
"Peer":{"nodekey:b":{"ID":"nB","HostName":"zeta","TailscaleIPs":["fd7a:115c:a1e0::3","100.64.0.3"],"Online":true,"ExitNodeOption":true,"ExitNode":true},
"nodekey:a":{"ID":"nA","HostName":"alpha","TailscaleIPs":["100.64.0.2"],"ExitNodeOption":true},
"nodekey:c":{"ID":"nC","HostName":"phone","TailscaleIPs":["100.64.0.4"]}}}`

func TestParseRunningListsExitNodesSorted(t *testing.T) {
	prefs := `{"ExitNodeID":"nB","ExitNodeIP":"","CorpDNS":false,"RunSSH":false,"WantRunning":true,"AdvertiseRoutes":null}`
	b, err := Parse([]byte(runningJSON), []byte(prefs))
	if err != nil {
		t.Fatal(err)
	}
	if b.MagicDNS != (MagicDNS{Enabled: true, Suffix: "tail0000.ts.net"}) {
		t.Fatalf("magic %+v", b.MagicDNS)
	}
	if len(b.ExitNodes) != 2 || b.ExitNodes[0].ID != "nA" || b.ExitNodes[1].ID != "nB" || !b.ExitNodes[1].Active {
		t.Fatalf("exit nodes %+v", b.ExitNodes)
	}
	if b.Prefs.ExitNode != "nB" {
		t.Fatalf("exit node %q", b.Prefs.ExitNode)
	}
	if d := CurrentDesired(b); d.ExitNodeIP != "100.64.0.3" {
		t.Fatalf("exit node IP %q (want the IPv4 even when listed second)", d.ExitNodeIP)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	prefs := fixture(t, "prefs-default.json")
	for _, raw := range []string{"", "{", `{"Version":"x"}`} {
		if _, err := Parse([]byte(raw), prefs); err == nil {
			t.Fatalf("%q parsed", raw)
		}
	}
	if _, err := Parse(fixture(t, "status-needslogin.json"), []byte("Logged out.")); err == nil {
		t.Fatal("text prefs parsed")
	}
}

func TestFlagsAlwaysPinDNSOff(t *testing.T) {
	got := Flags(Desired{})
	want := []string{"--accept-dns=false", "--exit-node=", "--exit-node-allow-lan-access=false", "--advertise-routes=", "--ssh=false"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
	got = LoginArgs(Desired{ExitNodeIP: "100.64.0.2", AdvertiseRoutes: []string{"192.168.8.0/24", "10.0.0.0/8"}, SSH: true})
	want = []string{"--reset", "--accept-dns=false", "--exit-node=100.64.0.2", "--exit-node-allow-lan-access=true", "--advertise-routes=192.168.8.0/24,10.0.0.0/8", "--ssh=true"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestCurrentDesiredDropsUnknownExitNode(t *testing.T) {
	b := Backend{Prefs: Prefs{ExitNode: "nGone"}, ExitNodes: []ExitNode{{Node: Node{ID: "nA", IPs: []string{"100.64.0.2"}}}}}
	if d := CurrentDesired(b); d.ExitNodeIP != "" {
		t.Fatalf("guessed %q", d.ExitNodeIP)
	}
}
