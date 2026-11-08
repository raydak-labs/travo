package uci

import (
	"slices"
	"testing"
)

func TestSplitListValue(t *testing.T) {
	cases := map[string][]string{
		"":              nil,
		"wan":           {"wan"},
		"wan,wan6,wwan": {"wan", "wan6", "wwan"},
		"wan wan6":      {"wan", "wan6"},
		"wan, wan6 ,":   {"wan", "wan6"},
	}
	for in, want := range cases {
		if got := SplitListValue(in); !slices.Equal(got, want) {
			t.Errorf("SplitListValue(%q) = %v, want %v", in, got, want)
		}
	}
}
