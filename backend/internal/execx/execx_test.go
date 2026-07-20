package execx

import (
	"strings"
	"testing"
	"time"
)

func TestOutput_ReturnsStdout(t *testing.T) {
	out, err := Output(Quick, "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(string(out)) != "hello" {
		t.Errorf("expected 'hello', got %q", out)
	}
}

func TestOutput_KillsHangingCommand(t *testing.T) {
	start := time.Now()
	_, err := Output(100*time.Millisecond, "sleep", "10")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed > 3*time.Second {
		t.Errorf("expected command killed near the deadline, took %v", elapsed)
	}
}

func TestCombinedOutput_IncludesStderr(t *testing.T) {
	out, err := CombinedOutput(Quick, "sh", "-c", "echo err >&2; echo out")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "err") || !strings.Contains(s, "out") {
		t.Errorf("expected stderr and stdout merged, got %q", s)
	}
}

func TestRun_ReturnsErrorForFailingCommand(t *testing.T) {
	if err := Run(Quick, "sh", "-c", "exit 3"); err == nil {
		t.Error("expected error for non-zero exit")
	}
}

func TestRun_KillsHangingCommand(t *testing.T) {
	start := time.Now()
	err := Run(100*time.Millisecond, "sleep", "10")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if time.Since(start) > 3*time.Second {
		t.Error("expected command killed near the deadline")
	}
}

func TestStream_SendsLinesAndTimesOut(t *testing.T) {
	var lines []string
	err := Stream(Quick, func(line string) { lines = append(lines, line) }, "sh", "-c", "echo one; echo two")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 2 || lines[0] != "one" || lines[1] != "two" {
		t.Errorf("expected [one two], got %v", lines)
	}

	start := time.Now()
	err = Stream(100*time.Millisecond, func(string) {}, "sh", "-c", "echo x; sleep 10")
	if err == nil {
		t.Fatal("expected timeout error from streaming command")
	}
	if time.Since(start) > 3*time.Second {
		t.Error("expected streaming command killed near the deadline")
	}
}
