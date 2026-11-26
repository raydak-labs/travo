package cli

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/device"
	"github.com/blackdark-org/travo-greenfield/internal/runner"
)

type call struct{ name, args string }

func failedCommand(name, diagnostic string, args ...string) error {
	return &runner.CommandError{
		Name:   name,
		Args:   args,
		Err:    errors.New("exit status 1"),
		Stderr: diagnostic,
	}
}

func fake(calls *[]call, reply map[string]string, failCmd string) runner.Runner {
	return func(ctx context.Context, name string, args ...string) (string, int, error) {
		a := strings.Join(args, " ")
		*calls = append(*calls, call{name, a})
		if _, ok := ctx.Deadline(); !ok {
			panic("shell-out without a timeout tier")
		}
		if failCmd != "" && strings.Contains(a, failCmd) {
			return "", 1, errors.New("exit 1")
		}
		return reply[name+" "+a], 0, nil
	}
}

func TestUCIExitOneNotFoundClassification(t *testing.T) {
	for _, tc := range []struct {
		name         string
		args         []string
		diagnostic   string
		wantNotFound bool
	}{
		{
			name:         "export entry not found diagnostic",
			args:         []string{"export", "nosuch"},
			diagnostic:   "uci: Entry not found",
			wantNotFound: true,
		},
		{
			name:         "export transient failure",
			args:         []string{"export", "network"},
			diagnostic:   "uci: I/O error",
			wantNotFound: false,
		},
		{
			name:         "different diagnostic containing entry not found",
			args:         []string{"export", "network"},
			diagnostic:   "uci: Entry not found while reading config",
			wantNotFound: false,
		},
		{
			name:         "quiet get suppresses not found diagnostic",
			args:         []string{"-q", "get", "network.nosuch"},
			diagnostic:   "",
			wantNotFound: true,
		},
		{
			name:         "quiet delete suppresses not found diagnostic",
			args:         []string{"-q", "delete", "dhcp.nosuch.server"},
			diagnostic:   "",
			wantNotFound: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &Device{Run: func(context.Context, string, ...string) (string, int, error) {
				return "", 1, failedCommand("uci", tc.diagnostic, tc.args...)
			}}
			_, err := d.run(context.Background(), TierRead, "uci", tc.args...)
			if err == nil {
				t.Fatal("exit 1 returned no error")
			}
			if got := errors.Is(err, device.ErrNotFound); got != tc.wantNotFound {
				t.Fatalf("errors.Is(ErrNotFound) = %v, want %v: %v", got, tc.wantNotFound, err)
			}
		})
	}
}

func TestSetRefusesMissingSection(t *testing.T) {
	var calls []call
	d := &Device{Run: fake(&calls, nil, "-q get")}
	err := d.Set(context.Background(), "network", "nope", "proto", "dhcp")
	if !errors.Is(err, device.ErrNotFound) {
		t.Fatalf("got %v", err)
	}
	for _, c := range calls {
		if strings.Contains(c.args, "set ") {
			t.Fatal("wrote despite missing section")
		}
	}
}

func TestEveryCallCarriesDeadline(t *testing.T) {
	var calls []call
	d := &Device{Run: fake(&calls, map[string]string{"ubus call network.device status {}": `{"up":true}`}, "")}
	ctx := context.Background()
	_ = d.SetType(ctx, "network", "wwan", "interface")
	_ = d.AddToList(ctx, "dhcp", "x", "server", "1.1.1.1")
	_ = d.Commit(ctx, "network")
	if _, err := d.Call(ctx, "network.device", "status", nil); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 4 {
		t.Fatalf("%v", calls)
	}
}

func TestCallRejectsNonJSON(t *testing.T) {
	var calls []call
	d := &Device{Run: fake(&calls, map[string]string{"ubus call a.b c {}": "oops"}, "")}
	if _, err := d.Call(context.Background(), "a.b", "c", nil); err == nil {
		t.Fatal("accepted non-JSON reply")
	}
}

func TestCallDoesNotMapDifferentDiagnosticContainingNotFound(t *testing.T) {
	d := &Device{Run: func(context.Context, string, ...string) (string, int, error) {
		return "", 1, failedCommand("ubus", "Command failed: Not found; retry later")
	}}
	_, err := d.Call(context.Background(), "network.interface.wg0", "status", nil)
	if errors.Is(err, device.ErrNotFound) {
		t.Fatalf("unrecognized diagnostic mapped to ErrNotFound: %v", err)
	}
}

func TestDefaultRouteReadsKernelTable(t *testing.T) {
	var calls []call
	d := &Device{Run: fake(&calls, map[string]string{"ip -4 route show default": "default via 192.168.8.1 dev wlan0\n"}, "")}
	ok, err := d.HasDefaultRoute(context.Background())
	if err != nil || !ok {
		t.Fatalf("%v %v", ok, err)
	}
	d2 := &Device{Run: fake(&calls, nil, "")}
	if ok, _ := d2.HasDefaultRoute(context.Background()); ok {
		t.Fatal("empty table reported as route")
	}
}

func TestExecRunnerHonoursTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _, err := runner.Exec(ctx, "sleep", "5")
	if err == nil || time.Since(start) > 2*time.Second {
		t.Fatalf("timeout not enforced: %v after %v", err, time.Since(start))
	}
}
