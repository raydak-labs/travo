package crontab

import (
	"errors"
	"testing"
)

const foreign = "# operator job\n*/5 * * * * /usr/bin/check.sh\n"

func TestReplaceKeepsForeignLines(t *testing.T) {
	got, err := Replace(foreign, "leds", []string{"0 23 * * * a", "30 6 * * * b"})
	if err != nil {
		t.Fatal(err)
	}
	want := foreign + "# travo-greenfield:leds begin\n0 23 * * * a\n30 6 * * * b\n# travo-greenfield:leds end\n"
	if got != want {
		t.Fatalf("got %q", got)
	}
	// replacing again swaps only the block, another owner's block stays
	two, _ := Replace(got, "wifi", []string{"0 1 * * * w"})
	three, _ := Replace(two, "leds", []string{"5 22 * * * c"})
	if l := Lines(three, "wifi"); len(l) != 1 || l[0] != "0 1 * * * w" {
		t.Fatalf("wifi block %q", l)
	}
	if l := Lines(three, "leds"); len(l) != 1 || l[0] != "5 22 * * * c" {
		t.Fatalf("leds block %q", l)
	}
	if three[:len(foreign)] != foreign {
		t.Fatalf("foreign lines changed: %q", three)
	}
	// removing the last block restores the original bytes
	a, _ := Replace(three, "leds", nil)
	b, _ := Replace(a, "wifi", nil)
	if b != foreign {
		t.Fatalf("got %q", b)
	}
}

func TestReplaceEmpty(t *testing.T) {
	got, err := Replace("", "leds", nil)
	if err != nil || got != "" {
		t.Fatalf("%q %v", got, err)
	}
	got, _ = Replace("# travo-greenfield:leds begin\n0 1 * * * x\n# travo-greenfield:leds end\n", "leds", nil)
	if got != "" {
		t.Fatalf("%q", got)
	}
}

func TestReplaceDropsAnUnterminatedOwnBlock(t *testing.T) {
	got, _ := Replace("1 1 * * * keep\n# travo-greenfield:leds begin\n0 1 * * * x\n", "leds", nil)
	if got != "1 1 * * * keep\n" {
		t.Fatalf("%q", got)
	}
}

func TestReplaceRejectsBadInput(t *testing.T) {
	for _, tc := range []struct {
		owner string
		lines []string
	}{
		{"Bad Owner", nil},
		{"leds", []string{"0 1 * * * a\n* * * * * evil"}},
		{"leds", []string{"0 1 * *"}},
		{"leds", []string{"# comment only here ok"}},
	} {
		if _, err := Replace("", tc.owner, tc.lines); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q %q: %v", tc.owner, tc.lines, err)
		}
	}
}

func TestClockAndDaily(t *testing.T) {
	c, err := ParseClock("06:30")
	if err != nil || c != (Clock{6, 30}) || c.String() != "06:30" {
		t.Fatalf("%v %v", c, err)
	}
	for _, bad := range []string{"6:30", "24:00", "12:60", "ab:cd", "", "12:3"} {
		if _, err := ParseClock(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	line := Daily(c, HookCommand("leds-off"))
	if line != "30 6 * * * /usr/bin/travo-greenfield --hook leds-off 2>&1 | logger -t travo-greenfield-hook" {
		t.Fatalf("%q", line)
	}
	back, cmd, ok := ParseDaily(line)
	if !ok || back != c || cmd != HookCommand("leds-off") {
		t.Fatalf("%v %q %v", back, cmd, ok)
	}
	for _, bad := range []string{"*/5 * * * * x", "0 1 * * 1 x", "0 25 * * * x", "0 1 * *"} {
		if _, _, ok := ParseDaily(bad); ok {
			t.Errorf("%q parsed", bad)
		}
	}
}
