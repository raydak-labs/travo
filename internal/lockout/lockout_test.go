package lockout

import (
	"errors"
	"net/netip"
	"testing"
)

// Shapes follow docs/device-facts.md §5; values are illustrative until spike S3 captures land.
var facts = Facts{
	Neighbours: map[netip.Addr]string{
		netip.MustParseAddr("192.168.1.10"): "AA:BB:CC:00:00:10", // phone on AP
		netip.MustParseAddr("192.168.1.20"): "aa:bb:cc:00:00:20", // laptop on cable
		netip.MustParseAddr("192.168.1.30"): "",                  // incomplete neighbour entry
	},
	Stations:   map[string][]string{"phy1-ap0": {"aa:bb:cc:00:00:10"}, "phy0-ap0": {}},
	EnabledAPs: []string{"phy0-ap0", "phy1-ap0"},
	BridgeNets: []netip.Prefix{netip.MustParsePrefix("192.168.1.0/24")},
	UplinkNets: []netip.Prefix{netip.MustParsePrefix("10.20.0.0/16")},
}

func TestClassify(t *testing.T) {
	cases := []struct {
		ip   string
		want Caller
	}{
		{"192.168.1.10", Caller{Method: WifiAP, AP: "phy1-ap0"}},
		{"192.168.1.20", Caller{Method: Ethernet}},
		{"192.168.1.30", Caller{Method: Unknown}}, // no MAC: same /24 cannot tell wired from WiFi
		{"192.168.1.99", Caller{Method: Unknown}}, // not in neighbour table
		{"10.20.3.4", Caller{Method: WifiClient}},
		{"8.8.8.8", Caller{Method: Unknown}},
		{"::ffff:192.168.1.10", Caller{Method: WifiAP, AP: "phy1-ap0"}},
	}
	for _, c := range cases {
		if got := Classify(netip.MustParseAddr(c.ip), facts); got != c.want {
			t.Errorf("%s: got %+v want %+v", c.ip, got, c.want)
		}
	}
	if got := Classify(netip.Addr{}, facts); got.Method != Unknown {
		t.Error("invalid addr")
	}
}

var (
	main5 = AP{Ifname: "phy0-ap0", Section: "default_radio0", Enabled: true}
	main2 = AP{Ifname: "phy1-ap0", Section: "default_radio1", Enabled: true}
	guest = AP{Ifname: "phy1-ap1", Section: "guest", Enabled: true, Guest: true}
	off   = func(a AP) AP { a.Enabled = false; return a }
	onAP  = Caller{Method: WifiAP, AP: "phy1-ap0"}
)

func TestCheckMatrix(t *testing.T) {
	before := []AP{main5, main2, guest}
	cases := []struct {
		name   string
		caller Caller
		after  []AP
		block  bool
	}{
		// MUST block (invariants §2)
		{"mode to client removes all APs", onAP, nil, true},
		{"disable all radios", onAP, []AP{off(main5), off(main2), off(guest)}, true},
		{"only guest left for main-AP caller", onAP, []AP{off(main5), off(main2), guest}, true},
		{"unknown caller loses all non-guest", Caller{Method: Unknown}, []AP{off(main5), off(main2), guest}, true},
		// MUST NOT block
		{"SSID/key change keeps own AP", onAP, []AP{main5, main2, guest}, false},
		{"disable other radio while own AP up", onAP, []AP{off(main5), main2, guest}, false},
		{"disable own radio while other AP up", onAP, []AP{main5, off(main2), guest}, false},
		{"disable guest while main up", onAP, []AP{main5, main2, off(guest)}, false},
		{"ethernet caller may remove everything", Caller{Method: Ethernet}, nil, false},
		{"unknown caller keeps a main AP", Caller{Method: Unknown}, []AP{main5, off(main2)}, false},
	}
	for _, c := range cases {
		err := Check(c.caller, Plan{Before: before, After: c.after}, false)
		if c.block != errors.Is(err, ErrLockoutRisk) {
			t.Errorf("%s: block=%v err=%v", c.name, c.block, err)
		}
	}
}

func TestGuestCallerKeepsOwnGuestAP(t *testing.T) {
	c := Caller{Method: WifiAP, AP: "phy1-ap1"}
	before := []AP{main5, guest}
	if err := Check(c, Plan{Before: before, After: []AP{off(main5), guest}}, false); err != nil {
		t.Fatal("guest caller on surviving guest AP blocked", err)
	}
	if err := Check(c, Plan{Before: before, After: []AP{off(main5), off(guest)}}, false); !errors.Is(err, ErrLockoutRisk) {
		t.Fatal("not blocked")
	}
}

func TestAcknowledgeOverrides(t *testing.T) {
	if err := Check(onAP, Plan{}, true); err != nil {
		t.Fatal(err)
	}
}

func TestMissingStationDumpFailsClosed(t *testing.T) {
	ip := netip.MustParseAddr("192.168.1.20") // known MAC, not seen on any readable AP
	partial := facts
	partial.Stations = map[string][]string{"phy1-ap0": {"aa:bb:cc:00:00:10"}} // phy0-ap0 dump failed
	if got := Classify(ip, partial); got.Method != Unknown {
		t.Fatalf("partial dump classified as %v", got)
	}
	noList := facts
	noList.EnabledAPs = nil // stations present, completeness unknown
	if got := Classify(ip, noList); got.Method != Unknown {
		t.Fatalf("missing EnabledAPs classified as %v", got)
	}
	none := facts
	none.Stations, none.EnabledAPs = nil, nil
	c := Classify(ip, none)
	if c.Method != Unknown {
		t.Fatalf("no AP facts classified as %v", c)
	}
	if err := Check(c, Plan{After: []AP{off(main5), off(main2)}}, false); !errors.Is(err, ErrLockoutRisk) {
		t.Fatal("unknown caller not blocked")
	}
}
