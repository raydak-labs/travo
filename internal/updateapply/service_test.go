package updateapply

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/blackdark-org/travo-greenfield/internal/guard"
)

func fixtureService(t *testing.T) *Service {
	t.Helper()
	root := t.TempDir()
	return &Service{layout: layout{root: root, binary: filepath.Join(root, "binary"), ui: filepath.Join(root, "www"), manifest: filepath.Join(root, "manifest"), lock: filepath.Join(root, "lock")}, guards: guard.Dir{Path: filepath.Join(root, "guards"), Now: time.Now}}
}

func TestHealthEndpoint(t *testing.T) {
	for _, tt := range []struct{ listen, want string }{
		{":8090", "http://127.0.0.1:8090/api/health"},
		{"0.0.0.0:8091", "http://127.0.0.1:8091/api/health"},
		{"[::]:80", "http://[::1]:80/api/health"},
		{"[2001:db8::5]:8080", "http://[2001:db8::5]:8080/api/health"},
		{"192.168.1.1:80", "http://192.168.1.1:80/api/health"},
		{"localhost:80", ""}, {":0", ""},
	} {
		got, err := healthEndpoint(tt.listen)
		if got != tt.want || (err != nil) != (tt.want == "") {
			t.Errorf("%s: %s %v", tt.listen, got, err)
		}
	}
}

func TestInvalidJournalRetainsGuard(t *testing.T) {
	s := fixtureService(t)
	if err := s.guards.Acquire(GuardName); err != nil {
		t.Fatal(err)
	}
	j := journal{ApplyStatus: ApplyStatus{ID: strings.Repeat("a", 32), Version: "1.2.3", State: "succeeded"}, Phase: "switching", PreviousVersion: "1.2.2", Endpoint: "http://127.0.0.1:80/api/health"}
	if err := s.save(j); err != nil {
		t.Fatal(err)
	}
	if err := s.work(nil); err == nil {
		t.Fatal("accepted inconsistent state")
	}
	if active, err := s.guarded(); !active || err != nil {
		t.Fatal("lost recovery guard", err)
	}
}

func TestInheritedAcknowledgement(t *testing.T) {
	if os.Getenv("TRAVO_TEST_ACK_CHILD") == "yes" {
		if err := makeAckNonblocking(); err != nil {
			t.Fatal(err)
		}
		pipe := os.NewFile(4, "test acknowledgement")
		defer pipe.Close()
		if !acknowledge(pipe) {
			t.Fatal("did not accept acknowledgement")
		}
		return
	}
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	defer write.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestInheritedAcknowledgement$")
	cmd.Env = append(os.Environ(), "TRAVO_TEST_ACK_CHILD=yes")
	cmd.ExtraFiles = []*os.File{os.Stdin, read}
	if _, err = write.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}

func TestWaitsForOriginalProcess(t *testing.T) {
	if _, err := os.Stat("/proc/self/stat"); errors.Is(err, os.ErrNotExist) {
		t.Skip("requires procfs")
	}
	child := exec.Command("sleep", "10")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer child.Wait()
	defer child.Process.Kill()
	process, err := processAt(child.Process.Pid)
	if err != nil || process.PID == 0 {
		t.Fatal(process, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if err := waitProcess(ctx, process); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}

type exitRouter struct {
	pids  []int
	stops int
}

func (r *exitRouter) UpdateSettings(context.Context) (string, string, error) { return "", "", nil }
func (r *exitRouter) StartUpdateService(context.Context) error               { return nil }
func (r *exitRouter) StopUpdateService(context.Context) error {
	r.stops++
	return nil
}
func (r *exitRouter) UpdatePID(context.Context) (int, error) {
	if len(r.pids) == 0 {
		return 0, nil
	}
	pid := r.pids[0]
	if len(r.pids) > 1 {
		r.pids = r.pids[1:]
	}
	return pid, nil
}

func TestAwaitInstanceExitAcceptsStaleListing(t *testing.T) {
	router := &exitRouter{pids: []int{9, 9, 0}}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := awaitInstanceExit(ctx, router, processIdentity{PID: 9, Start: "1"}, func(int) (processIdentity, error) {
		return processIdentity{}, nil
	})
	if err != nil || router.stops != 0 {
		t.Fatal(err, router.stops)
	}
}

func TestAwaitInstanceExitStopsRespawn(t *testing.T) {
	const respawned = 1 << 30 // not a live pid; waitProcess must observe it as gone
	router := &exitRouter{pids: []int{respawned, 0}}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := awaitInstanceExit(ctx, router, processIdentity{PID: 1, Start: "1"}, func(pid int) (processIdentity, error) {
		return processIdentity{PID: pid, Start: "2"}, nil
	})
	if err != nil || router.stops != 1 {
		t.Fatal(err, router.stops)
	}
}

func TestAwaitInstanceExitTimesOutWhileListed(t *testing.T) {
	router := &exitRouter{pids: []int{4}}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := awaitInstanceExit(ctx, router, processIdentity{PID: 4, Start: "1"}, func(int) (processIdentity, error) {
		return processIdentity{}, nil
	})
	if err == nil || err.Error() != "procd instance still running" || router.stops != 0 {
		t.Fatal(err, router.stops)
	}
}

func TestReadProcessAtHandlesExitedProcess(t *testing.T) {
	for _, tc := range []struct {
		name string
		read error
		want error
	}{
		{"missing", syscall.ENOENT, nil},
		{"exited during read", syscall.ESRCH, nil},
		{"permission denied", syscall.EACCES, syscall.EACCES},
	} {
		t.Run(tc.name, func(t *testing.T) {
			process, err := readProcessAt(2315, func(path string) ([]byte, error) {
				return nil, &os.PathError{Op: "read", Path: path, Err: tc.read}
			})
			if process != (processIdentity{}) || !errors.Is(err, tc.want) {
				t.Fatalf("process=%+v error=%v, want absent process and error %v", process, err, tc.want)
			}
		})
	}
}
