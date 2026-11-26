package cli

import (
	"os"
	"reflect"
	"slices"
	"testing"
)

func TestParseExportContainerFixture(t *testing.T) {
	b, err := os.ReadFile("testdata/uci-export-dhcp.txt")
	if err != nil {
		t.Fatal(err)
	}
	secs, err := ParseExport("dhcp", string(b))
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, s := range secs {
		names = append(names, s.Name)
	}
	if !reflect.DeepEqual(names, []string{"@dnsmasq[0]", "lan", "wan", "extra"}) {
		t.Fatal(names)
	}
	d := secs[0]
	if !reflect.DeepEqual(d.Lists["server"], []string{"1.1.1.1", "/lan/127.0.0.1#53"}) || d.Options["note"] != "it's quoted" || d.Options["dq"] != `a "b" $x` {
		t.Fatalf("%+v", d)
	}
}

// The device bug: a one-element list. `uci show` cannot tell it from an option.
func TestParseExportKeepsSingleElementList(t *testing.T) {
	b, err := os.ReadFile("../../../testdata/device/axt1800-25.12.6/uci-export-dhcp.txt")
	if err != nil {
		t.Fatal(err)
	}
	secs, err := ParseExport("dhcp", string(b))
	if err != nil {
		t.Fatal(err)
	}
	d := secs[0]
	if d.Name != "@dnsmasq[0]" || !reflect.DeepEqual(d.Lists["server"], []string{"127.0.0.1#5353"}) {
		t.Fatalf("%+v", d)
	}
	if _, isOpt := d.Options["server"]; isOpt {
		t.Fatal("list parsed as option")
	}
	show, _ := os.ReadFile("../../../testdata/device/axt1800-25.12.6/uci-show-dhcp-list-single.txt")
	viaShow, _ := ParseShow("dhcp", string(show))
	if _, isOpt := viaShow[0].Options["server"]; !isOpt {
		t.Fatal("expected uci show to lose the list-ness (documents why export is used)")
	}
}

func TestParseExportRejectsGarbage(t *testing.T) {
	for _, bad := range []string{
		"config x\n",                      // no package
		"package other\nconfig x\n",       // wrong package
		"package dhcp\noption a 'b'\n",    // option outside section
		"package dhcp\nconfig x\n\tfoo\n", // unknown keyword
		"package dhcp\nconfig x\n\toption a 'b\n",
	} {
		if _, err := ParseExport("dhcp", bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestParseExportRecordsOptionOrder(t *testing.T) {
	b, err := os.ReadFile("../../../testdata/device/axt1800-25.12.6/uci-export-dhcp.txt")
	if err != nil {
		t.Fatal(err)
	}
	secs, err := ParseExport("dhcp", string(b))
	if err != nil {
		t.Fatal(err)
	}
	var owned []string
	for _, n := range secs[0].Order {
		if n == "server" || n == "noresolv" || n == "rebind_protection" {
			owned = append(owned, n)
		}
	}
	if want := []string{"rebind_protection", "noresolv", "server"}; !slices.Equal(owned, want) {
		t.Fatalf("order %v, want %v (full %v)", owned, want, secs[0].Order)
	}
	if n := len(secs[0].Order); n != len(secs[0].Options)+len(secs[0].Lists) {
		t.Fatalf("order names %d options, section has %d", n, len(secs[0].Options)+len(secs[0].Lists))
	}
}
