package facts

import (
	"errors"
	"os"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../../testdata/device/axt1800-25.12.6/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseBoardFixture(t *testing.T) {
	got, err := ParseBoard(fixture(t, "system-board.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := Board{Hostname: "OpenWrt", Model: "GL.iNet GL-AXT1800", Kernel: "6.12.112",
		BoardName: "glinet,gl-axt1800", Release: "25.12.6", Revision: "r33311-981c978eff", Target: "qualcommax/ipq60xx"}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestParseBoardShape(t *testing.T) {
	for name, in := range map[string]string{
		"empty object":   `{}`,
		"no release":     `{"model":"m","kernel":"k"}`,
		"no version":     `{"model":"m","kernel":"k","release":{"revision":"r1"}}`,
		"empty model":    `{"model":"","kernel":"k","release":{"version":"1"}}`,
		"missing kernel": `{"model":"m","release":{"version":"1"}}`,
		"empty version":  `{"model":"m","kernel":"k","release":{"version":""}}`,
	} {
		if _, err := ParseBoard([]byte(in)); !errors.Is(err, ErrShape) {
			t.Errorf("%s: want ErrShape, got %v", name, err)
		}
	}
	if _, err := ParseBoard([]byte(`not json`)); err == nil {
		t.Error("garbage must fail")
	}
}

func TestParseSystemInfoFixture(t *testing.T) {
	got, err := ParseSystemInfo(fixture(t, "system-info.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Uptime != 224*time.Second {
		t.Errorf("uptime %v", got.Uptime)
	}
	if got.Load != [3]float64{25408.0 / 65536, 15872.0 / 65536, 6816.0 / 65536} {
		t.Errorf("load %v", got.Load)
	}
	if got.TmpAvail != 202480*1024 {
		t.Errorf("tmp avail %d", got.TmpAvail)
	}
	if got.MemTotal != 428646400 || got.MemAvailable != 149295104 {
		t.Errorf("memory %d %d", got.MemTotal, got.MemAvailable)
	}
	if got.RootTotal != 81616896 || got.RootAvail != 69513216 {
		t.Errorf("root %d %d", got.RootTotal, got.RootAvail)
	}
}

func TestParseSystemInfoShape(t *testing.T) {
	const ok = `"uptime":1,"load":[1,2,3],"memory":{"total":10,"available":5},"root":{"total":8,"avail":4}`
	if _, err := ParseSystemInfo([]byte(`{` + ok + `}`)); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	for name, in := range map[string]string{
		"empty object":       `{}`,
		"load of two":        `{"uptime":1,"load":[1,2],"memory":{"total":10,"available":5},"root":{"total":8,"avail":4}}`,
		"no uptime":          `{"load":[1,2,3],"memory":{"total":10,"available":5},"root":{"total":8,"avail":4}}`,
		"no available":       `{"uptime":1,"load":[1,2,3],"memory":{"total":10},"root":{"total":8,"avail":4}}`,
		"no memory total":    `{"uptime":1,"load":[1,2,3],"memory":{"available":5},"root":{"total":8,"avail":4}}`,
		"available > total":  `{"uptime":1,"load":[1,2,3],"memory":{"total":10,"available":11},"root":{"total":8,"avail":4}}`,
		"no root total":      `{"uptime":1,"load":[1,2,3],"memory":{"total":10,"available":5},"root":{"avail":4}}`,
		"no root avail":      `{"uptime":1,"load":[1,2,3],"memory":{"total":10,"available":5},"root":{"total":8}}`,
		"root avail > total": `{"uptime":1,"load":[1,2,3],"memory":{"total":10,"available":5},"root":{"total":8,"avail":9}}`,
	} {
		if _, err := ParseSystemInfo([]byte(in)); !errors.Is(err, ErrShape) {
			t.Errorf("%s: want ErrShape, got %v", name, err)
		}
	}
}

func TestFirmwareURL(t *testing.T) {
	b, err := os.ReadFile("../../../testdata/device/axt1800-25.12.6/system-board.json")
	if err != nil {
		t.Fatal(err)
	}
	if u, err := FirmwareURL(b); err != nil || u != "https://downloads.openwrt.org/" {
		t.Fatalf("%q %v", u, err)
	}
	if _, err := FirmwareURL([]byte(`{"release":{}}`)); err == nil {
		t.Fatal("missing url accepted")
	}
}
