package osupdate

import (
	"os"
	"testing"
)

func capturedIndex(t *testing.T) Index {
	t.Helper()
	b, err := os.ReadFile("../../testdata/device/axt1800-25.12.6/updates/versions.json")
	if err != nil {
		t.Fatal(err)
	}
	ix, err := ParseIndex(b)
	if err != nil {
		t.Fatal(err)
	}
	return ix
}

func TestCompareOnCapturedIndex(t *testing.T) {
	ix := capturedIndex(t)
	if ix.Stable != "25.12.6" {
		t.Fatalf("stable %q", ix.Stable)
	}
	for _, tc := range []struct {
		current           string
		inSeries, nextSer string
	}{
		{"25.12.6", "", ""},
		{"25.12.3", "25.12.6", ""},
		{"24.10.2", "24.10.8", "25.12.6"},
		{"23.05.0", "23.05.6", "24.10.8"},
		{"25.12.0-rc2", "25.12.6", ""},
		{"SNAPSHOT", "", ""},
	} {
		got := Compare(tc.current, ix)
		if got.InSeries != tc.inSeries || got.NextSeries != tc.nextSer || got.Current != tc.current {
			t.Errorf("%s: %+v", tc.current, got)
		}
	}
}

func TestParseVersion(t *testing.T) {
	for s, ok := range map[string]bool{"25.12.6": true, "19.07.10": true, "25.12.0-rc1": false, "SNAPSHOT": false, "25.12": false, "25.+1.0": false} {
		if _, got := ParseVersion(s); got != ok {
			t.Errorf("%s: %v", s, got)
		}
	}
	a, _ := ParseVersion("19.07.10")
	b, _ := ParseVersion("19.07.9")
	if !b.Less(a) || a.String() != "19.07.10" || a.Series() != "19.07" {
		t.Fatal("order")
	}
}

func TestParseIndexRejects(t *testing.T) {
	for _, b := range []string{"{", `{"versions_list":[]}`} {
		if _, err := ParseIndex([]byte(b)); err == nil {
			t.Errorf("%s accepted", b)
		}
	}
}
