package uci

import "testing"

// Real `uci add_list` APPENDS, and a real `uci get` on a list option returns
// the values space separated on one line. The mock used to OVERWRITE, so code
// that builds a multi-entry list (mwan3 use_member, wg allowed_ips, a firewall
// zone network list) looked correct in tests and lost every entry but the last on
// the device.
func TestMockUCIAddListAppends(t *testing.T) {
	m := NewMockUCI()
	if err := m.AddList("firewall", "zone_wan", "network", "wwan"); err != nil {
		t.Fatalf("first add_list: %v", err)
	}
	if err := m.AddList("firewall", "zone_wan", "network", "travo_wan"); err != nil {
		t.Fatalf("second add_list: %v", err)
	}

	opts, err := m.GetAll("firewall", "zone_wan")
	if err != nil {
		t.Fatalf("get_all: %v", err)
	}
	want := "wan wan6 wwan travo_wan"
	if opts["network"] != want {
		t.Errorf("GetAll network = %q, want %q", opts["network"], want)
	}

	got, err := m.Get("firewall", "zone_wan", "network")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != want {
		t.Errorf("Get network = %q, want %q (both must agree with `uci get`)", got, want)
	}
}

// Appending must also build a list on an option that did not exist before, and
// must not disturb the neighbouring options.
func TestMockUCIAddListBuildsNewOption(t *testing.T) {
	m := NewMockUCI()
	if err := m.AddSection("mwan3", "travo_policy", "policy"); err != nil {
		t.Fatalf("add section: %v", err)
	}
	if err := m.Set("mwan3", "travo_policy", "dest_ip", "default"); err != nil {
		t.Fatalf("set: %v", err)
	}
	for _, member := range []string{"travo_wan_p10", "travo_wwan_p20"} {
		if err := m.AddList("mwan3", "travo_policy", "use_member", member); err != nil {
			t.Fatalf("add_list %s: %v", member, err)
		}
	}

	opts, err := m.GetAll("mwan3", "travo_policy")
	if err != nil {
		t.Fatalf("get_all: %v", err)
	}
	if want := "travo_wan_p10 travo_wwan_p20"; opts["use_member"] != want {
		t.Errorf("use_member = %q, want %q", opts["use_member"], want)
	}
	if opts["dest_ip"] != "default" {
		t.Errorf("dest_ip = %q, want it untouched", opts["dest_ip"])
	}
	if opts[".type"] != "policy" {
		t.Errorf(".type = %q, want policy", opts[".type"])
	}
}

// Set replaces a list wholesale, exactly as a real `uci set` does: it must not
// append to what AddList accumulated.
func TestMockUCISetReplacesList(t *testing.T) {
	m := NewMockUCI()
	if err := m.AddList("network", "wg0", "addresses", "10.0.0.3/24"); err != nil {
		t.Fatalf("add_list: %v", err)
	}
	if err := m.Set("network", "wg0", "addresses", "10.0.0.2/24"); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := m.Get("network", "wg0", "addresses")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != "10.0.0.2/24" {
		t.Errorf("addresses = %q, want the Set value only", got)
	}
}
