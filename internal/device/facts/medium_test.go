package facts_test

import (
	"os"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/facts"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
)

func TestClassifyMediumAXT1800Capture(t *testing.T) {
	b, err := os.ReadFile("../../../testdata/device/axt1800-25.12.6/sysfs-net/links.tsv")
	if err != nil {
		t.Fatal(err)
	}
	links, err := mock.ParseLinksTSV(b)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]facts.Medium{
		"br-guest": facts.MediumVirtual, "br-lan": facts.MediumVirtual, "lo": facts.MediumVirtual, "miireg": facts.MediumVirtual,
		"lan1": facts.MediumEthernet, "lan2": facts.MediumEthernet, "wan": facts.MediumEthernet,
		"phy0-sta0": facts.MediumWiFi, "phy1-ap0": facts.MediumWiFi, "phy1-ap1": facts.MediumWiFi,
	}
	if len(links) != len(want) {
		t.Fatalf("got %d links, want %d", len(links), len(want))
	}
	for _, l := range links {
		if got := facts.ClassifyMedium(l); got != want[l.Name] {
			t.Errorf("%s: %s, want %s", l.Name, got, want[l.Name])
		}
		if _, ok := facts.Tether(l); ok {
			t.Errorf("%s classified as a tether link", l.Name)
		}
	}
}

// No phone capture exists yet (hardware playbook): these cases pin the contract on the
// kernel driver names in drivers/net/usb, with the names a phone link may get.
func TestTetherByDriverNotName(t *testing.T) {
	cases := []struct {
		link device.NetLink
		kind facts.TetherKind
		ok   bool
	}{
		{device.NetLink{Name: "usb0", Bus: "usb", Driver: "rndis_host"}, facts.TetherRNDIS, true},
		{device.NetLink{Name: "usb0", Bus: "usb", Driver: "cdc_ncm"}, facts.TetherNCM, true},
		{device.NetLink{Name: "eth1", Bus: "usb", Driver: "cdc_ether"}, facts.TetherECM, true},
		{device.NetLink{Name: "eth0", Bus: "usb", Driver: "ipheth"}, facts.TetherIPheth, true},
		{device.NetLink{Name: "eth2", Bus: "usb", Driver: "r8152"}, "", false},           // USB Ethernet adapter
		{device.NetLink{Name: "usb0", Bus: "platform", Driver: "rndis_host"}, "", false}, // not on USB
		{device.NetLink{Name: "usb0"}, "", false},                                        // a name alone proves nothing
	}
	for _, c := range cases {
		k, ok := facts.Tether(c.link)
		if k != c.kind || ok != c.ok {
			t.Errorf("%+v: (%q,%v), want (%q,%v)", c.link, k, ok, c.kind, c.ok)
		}
		if c.link.Bus == "usb" && facts.ClassifyMedium(c.link) != facts.MediumUSB {
			t.Errorf("%+v: medium not usb", c.link)
		}
	}
}
