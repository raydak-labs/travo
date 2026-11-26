package cli

import (
	"os"
	"reflect"
	"testing"
)

func TestParseShowDHCPFixture(t *testing.T) {
	b, err := os.ReadFile("testdata/uci-show-dhcp.txt")
	if err != nil {
		t.Fatal(err)
	}
	secs, err := ParseShow("dhcp", string(b))
	if err != nil {
		t.Fatal(err)
	}
	if len(secs) != 3 || secs[0].Name != "@dnsmasq[0]" || secs[0].Type != "dnsmasq" {
		t.Fatalf("sections %+v", secs)
	}
	dns := secs[0]
	if want := []string{"1.1.1.1", "/lan/127.0.0.1#53"}; !reflect.DeepEqual(dns.Lists["server"], want) {
		t.Errorf("server list %v", dns.Lists["server"])
	}
	if dns.Options["note"] != "it's quoted" {
		t.Errorf("escape: %q", dns.Options["note"])
	}
	if dns.Options["rebind_protection"] != "1" {
		t.Errorf("option: %v", dns.Options)
	}
	if secs[1].Name != "lan" || secs[1].Options["start"] != "100" {
		t.Errorf("lan %+v", secs[1])
	}
}

func TestParseShowRejectsOtherConfigAndGarbage(t *testing.T) {
	if _, err := ParseShow("dhcp", "network.lan=interface"); err == nil {
		t.Error("accepted foreign key")
	}
	if _, err := ParseShow("dhcp", "dhcp.lan.start='1"); err == nil {
		t.Error("accepted unterminated quote")
	}
	if _, err := ParseShow("dhcp", "nonsense"); err == nil {
		t.Error("accepted malformed line")
	}
}
