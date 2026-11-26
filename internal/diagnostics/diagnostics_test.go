package diagnostics

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/device/mock"
)

type blockingDiagnoser struct {
	started, release chan struct{}
	output           string
	err              error
	calls            int
}

func (b *blockingDiagnoser) Diagnose(context.Context, string, string) (device.DiagnosticRun, error) {
	b.calls++
	if b.started != nil {
		close(b.started)
		<-b.release
	}
	return device.DiagnosticRun{Output: b.output, OK: true}, b.err
}

func TestRunWithMockCaptures(t *testing.T) {
	s := &Service{Device: mock.New()}
	res, err := s.Run(context.Background(), "ping", "10.0.1.1")
	if err != nil || !res.OK || res.Truncated || !strings.Contains(res.Output, "4 packets received") {
		t.Fatalf("%+v %v", res, err)
	}
	res, err = s.Run(context.Background(), "lookup", "does-not-exist.invalid")
	if err != nil || res.OK || !strings.Contains(res.Output, "NXDOMAIN") || strings.Contains(res.Output, "exit=") {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestRunRefusesBadInputBeforeTheDevice(t *testing.T) {
	d := &blockingDiagnoser{}
	s := &Service{Device: d}
	for _, tc := range [][2]string{{"ping", "-c100"}, {"ping", "x; reboot"}, {"nmap", "10.0.1.1"}, {"", "10.0.1.1"}, {"lookup", ""}} {
		if _, err := s.Run(context.Background(), tc[0], tc[1]); !errors.Is(err, ErrInvalid) {
			t.Fatalf("%v: %v", tc, err)
		}
	}
	if d.calls != 0 {
		t.Fatal("device ran")
	}
}

func TestOneRunAtATime(t *testing.T) {
	d := &blockingDiagnoser{started: make(chan struct{}), release: make(chan struct{})}
	s := &Service{Device: d}
	done := make(chan error)
	go func() {
		_, err := s.Run(context.Background(), "traceroute", "10.0.1.1")
		done <- err
	}()
	<-d.started
	if _, err := s.Run(context.Background(), "ping", "10.0.1.1"); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	close(d.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestOutputIsCappedOnARuneBoundary(t *testing.T) {
	d := &blockingDiagnoser{output: strings.Repeat("a", MaxOutput-1) + "é" + "tail"}
	res, err := (&Service{Device: d}).Run(context.Background(), "ping", "10.0.1.1")
	if err != nil || !res.Truncated || len(res.Output) != MaxOutput-1 {
		t.Fatalf("%v %v %d", err, res.Truncated, len(res.Output))
	}
	d.err = errors.New("exec: not found")
	if _, err := (&Service{Device: d}).Run(context.Background(), "ping", "10.0.1.1"); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}
