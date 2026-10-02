package execx

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// Reproduces the CI condition: a command producing several times the capacity
// of one pipe buffer, consumed a little more slowly than it is written, so the
// process can exit while output is still queued. Nothing may be lost.
//
// The per-line delay is deliberately small. Stream's command carries a WaitDelay
// (execx.waitDelay), after which os/exec abandons a slow consumer — so a
// pathologically slow consumer is a different failure mode, and a test that
// triggered it would be testing the timeout rather than the truncation.
//
// 5000 lines is enough: the pipe-based implementation this guards lost ~40% of
// them on the first attempt, and the whole test runs in well under a second.
func TestStream_NoLineLossUnderSlowConsumer(t *testing.T) {
	for attempt := range 2 {
		var mu sync.Mutex
		var got []string
		slow := func(line string) {
			time.Sleep(2 * time.Microsecond) // slower than the writer
			mu.Lock()
			got = append(got, line)
			mu.Unlock()
		}
		err := Stream(30*time.Second, slow, "sh", "-c",
			`i=0; while [ $i -lt 5000 ]; do echo "line $i"; i=$((i+1)); done`)
		if err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
		mu.Lock()
		n := len(got)
		first, last := "", ""
		if n > 0 {
			first, last = got[0], got[n-1]
		}
		mu.Unlock()
		if n != 5000 || first != "line 0" || last != "line 4999" {
			t.Fatalf("attempt %d: expected 20000 lines line 0..line 4999, got %d (%q..%q)", attempt, n, first, last)
		}
	}
}

func TestStream_HandlesTrailingLineWithoutNewline(t *testing.T) {
	var lines []string
	if err := Stream(5*time.Second, func(l string) { lines = append(lines, l) },
		"sh", "-c", `printf 'no-newline-tail'`); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 || !strings.HasSuffix(lines[0], "no-newline-tail") {
		t.Fatalf("expected the unterminated tail to be delivered, got %q", lines)
	}
}
