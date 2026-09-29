package execx

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// Reproduces the CI condition: a command producing far more output than one
// pipe buffer, with a slow consumer, so the process exits while output is still
// queued. Nothing may be lost.
func TestStream_NoLineLossUnderSlowConsumer(t *testing.T) {
	for attempt := range 5 {
		var mu sync.Mutex
		var got []string
		slow := func(line string) {
			time.Sleep(25 * time.Microsecond) // slow consumer
			mu.Lock()
			got = append(got, line)
			mu.Unlock()
		}
		err := Stream(30*time.Second, slow, "sh", "-c",
			`i=0; while [ $i -lt 4000 ]; do echo "line $i"; i=$((i+1)); done`)
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
		if n != 4000 || first != "line 0" || last != "line 3999" {
			t.Fatalf("attempt %d: expected 4000 lines line 0..line 3999, got %d (%q..%q)", attempt, n, first, last)
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
