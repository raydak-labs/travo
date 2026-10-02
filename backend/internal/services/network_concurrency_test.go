package services

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
)

// A UCI backend that behaves like the real CLI's process-global staged-changes
// file: /tmp/.uci/<config>/changes is shared, so two interleaved writers of the
// same config destroy each other. Unserialised, the second writer's `set`
// lands on a delta the first writer is about to commit, and the first
// writer's own section disappears.
//
// This is not hypothetical. On the device, 24 concurrent writers across
// AddDNSEntry / AddDHCPReservation (both `dhcp`) produced
// "uci set dhcp.dns_cc_w3_0.name: uci: Invalid argument" and
// "uci: Invalid argument" — each request failing on its OWN section, clobbered
// by the other request. AddDNSEntry was a documented gap in the mutateUCI
// rollout, and the gap was reachable from the UI.
// sharedChangesUCI models the part of the real uci CLI that makes concurrent
// writers unsafe: every `uci` invocation reads the whole staged-changes document
// for a config (/tmp/.uci/<config>/changes), mutates its own snapshot, and
// writes the document back. Two invocations that interleave between the read
// and the write therefore lose each other's staged sections — which is exactly
// what the device reported, as "uci: Invalid argument" on the loser's own
// `uci set`.
//
// A call that references a section present in neither the committed config nor
// its own snapshot returns uci's real error. That is the assertion this test
// turns on: a section the caller created a moment ago has vanished, so some
// other writer committed in between.
type sharedChangesUCI struct {
	docMu     sync.Mutex
	doc       map[string]map[string]bool // config -> sections staged right now
	committed map[string]bool            // "config.section" that survived a commit
}

func newRacyUCI() *sharedChangesUCI {
	return &sharedChangesUCI{
		doc:       map[string]map[string]bool{},
		committed: map[string]bool{},
	}
}

func (r *sharedChangesUCI) Get(string, string, string) (string, error) { return "", nil }
func (r *sharedChangesUCI) GetAll(string, string) (map[string]string, error) {
	return map[string]string{}, nil
}
func (r *sharedChangesUCI) GetSections(string) (map[string]map[string]string, error) {
	return map[string]map[string]string{}, nil
}
func (r *sharedChangesUCI) AddList(string, string, string, string) error { return nil }

// yield is the read/write window inside one `uci` invocation. A real fork+exec
// is far slower than this, so the window here is a conservative stand-in.
func (r *sharedChangesUCI) yield() { runtime.Gosched() }

// edit runs one uci mutation against a snapshot of the changes document.
func (r *sharedChangesUCI) edit(config string, fn func(staged map[string]bool) error) error {
	r.docMu.Lock()
	snapshot := make(map[string]bool, len(r.doc[config]))
	for k := range r.doc[config] {
		snapshot[k] = true
	}
	r.docMu.Unlock()

	r.yield() // another invocation can rewrite the document here

	if err := fn(snapshot); err != nil {
		return err
	}

	r.docMu.Lock()
	r.doc[config] = snapshot // blind write-back: concurrent updates are lost
	r.docMu.Unlock()
	return nil
}

// exists reports whether the section is in the snapshot or already committed.
func (r *sharedChangesUCI) exists(config, section string, staged map[string]bool) bool {
	if staged[section] {
		return true
	}
	r.docMu.Lock()
	defer r.docMu.Unlock()
	return r.committed[config+"."+section]
}

func (r *sharedChangesUCI) AddSection(config, section, _ string) error {
	return r.edit(config, func(staged map[string]bool) error {
		staged[section] = true
		return nil
	})
}

func (r *sharedChangesUCI) Set(config, section, option, _ string) error {
	return r.edit(config, func(staged map[string]bool) error {
		if !r.exists(config, section, staged) {
			return fmt.Errorf("uci set %s.%s.%s: uci: Invalid argument", config, section, option)
		}
		return nil
	})
}

func (r *sharedChangesUCI) Commit(config string) error {
	r.docMu.Lock()
	defer r.docMu.Unlock()
	for section := range r.doc[config] {
		r.committed[config+"."+section] = true
	}
	r.doc[config] = map[string]bool{}
	return nil
}

func (r *sharedChangesUCI) DeleteOption(config, section, option string) error {
	return r.edit(config, func(map[string]bool) error { return nil })
}

func (r *sharedChangesUCI) DeleteSection(config, section string) error {
	r.docMu.Lock()
	defer r.docMu.Unlock()
	delete(r.committed, config+"."+section)
	for k := range r.doc[config] {
		if k == section {
			delete(r.doc[config], k)
		}
	}
	return nil
}

// AddDNSEntry and AddDHCPReservation both write `dhcp`, so they must hold the
// same config lock. Without it they interleave and fail; with it, every entry
// survives.
func TestConcurrentDHCPWritersDoNotCorruptEachOther(t *testing.T) {
	const workers, perWorker = 8, 6

	u := newRacyUCI()
	svc := NewNetworkServiceWithRunner(u, nil, nil)

	var wg sync.WaitGroup
	errs := make(chan error, workers*perWorker*2)
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range perWorker {
				u.yield()
				name := fmt.Sprintf("probe-w%d-%d", w, i)
				if err := svc.AddDNSEntry(models.DNSEntry{Name: name, IP: "10.9.9.9"}); err != nil {
					errs <- fmt.Errorf("AddDNSEntry(%s): %w", name, err)
				}
				u.yield()
				if err := svc.AddDHCPReservation(models.DHCPReservation{
					Name: name, MAC: fmt.Sprintf("aa:bb:cc:dd:%02x:%02x", w, i), IP: "10.8.8.8",
				}); err != nil {
					errs <- fmt.Errorf("AddDHCPReservation(%s): %w", name, err)
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)

	failures := 0
	for err := range errs {
		failures++
		if failures <= 5 {
			t.Errorf("concurrent dhcp write failed: %v", err)
		}
	}
	if failures > 5 {
		t.Errorf("...and %d more failures", failures-5)
	}
}

// Port forwards are a read-modify-write over /etc/travo/port-forwards.json.
// Unserialised, two adds each read the same rule list and each wrote its own
// version, so one add vanished; and because os.WriteFile truncates before
// writing, a concurrent reader could see a half-written file.
//
// On the device: 35 successful POSTs, 1 rule on disk, plus
// "unexpected end of JSON input" 500s.
func TestConcurrentPortForwardAddsAreNotLost(t *testing.T) {
	const workers, perWorker = 8, 6

	dir := t.TempDir()
	pfFile := filepath.Join(dir, "port-forwards.json")
	u := newRacyUCI()
	svc := NewNetworkServiceWithRunner(u, nil, nil)
	svc.portForwardsFile = pfFile

	var wg sync.WaitGroup
	errs := make(chan error, workers*perWorker)
	for w := range workers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range perWorker {
				if err := svc.AddPortForward(models.PortForwardRule{
					Name:     fmt.Sprintf("rule-w%d-%d", w, i),
					Protocol: "tcp",
					SrcDPort: fmt.Sprintf("%d", 20000+w*10+i),
					DestIP:   "10.9.9.9", DestPort: "80", Enabled: true,
				}); err != nil {
					errs <- err
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("AddPortForward failed: %v", err)
	}

	rules, err := svc.GetPortForwards()
	if err != nil {
		t.Fatalf("GetPortForwards: %v", err)
	}
	if len(rules) != workers*perWorker {
		t.Errorf("got %d port-forward rules on disk, want %d: concurrent adds overwrote each other",
			len(rules), workers*perWorker)
	}

	// And the file must still be valid JSON after the storm.
	data, err := os.ReadFile(pfFile)
	if err != nil {
		t.Fatalf("read %s: %v", pfFile, err)
	}
	if !json.Valid(data) {
		t.Errorf("%s is not valid JSON after concurrent writes:\n%s", pfFile, data)
	}
}

// lockUCIConfigs sorts its inputs, so the acquisition order is global and
// identical on every path. That is what makes nesting safe: with a fixed order
// the wait-for graph is acyclic and deadlock is impossible.
//
// Without the sort, two flows that both need {network, firewall} but list them
// in opposite order deadlock the moment they meet — and on a router that takes
// the whole API down, because every other endpoint that touches a config queues
// on the same per-config locks. This test runs the two orders against each
// other repeatedly; before the sort it hangs.
func TestOverlappingUCIConfigLockSetsDoNotDeadlock(t *testing.T) {
	// Fresh config names, so this does not contend with other tests.
	const (
		alpha = "ddnslocktest-a"
		beta  = "ddnslocktest-b"
		gamma = "ddnslocktest-c"
	)

	orders := [][]string{
		{alpha, beta, gamma},
		{gamma, beta, alpha},
		{beta, gamma, alpha},
		{alpha, gamma, beta},
	}

	const rounds = 200
	done := make(chan struct{})
	for _, order := range orders {
		go func(configs []string) {
			defer func() { done <- struct{}{} }()
			for range rounds {
				unlock := lockUCIConfigs(configs...)
				// Hold briefly so the other orders genuinely interleave.
				runtime.Gosched()
				unlock()
			}
		}(order)
	}
	for range orders {
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			t.Fatal("overlapping lock sets deadlocked: lockUCIConfigs is not ordering acquisition globally")
		}
	}
}

// Passing the same config twice must not self-deadlock. mutateUCI callers
// legitimately pass a set, and a duplicate would otherwise try to lock one mutex
// twice.
func TestLockUCIConfigsToleratesDuplicates(t *testing.T) {
	const dup = "ddnslocktest-dup"
	done := make(chan struct{})
	go func() {
		defer close(done)
		unlock := lockUCIConfigs(dup, dup, dup)
		unlock()
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("lockUCIConfigs self-deadlocked on a duplicate config name")
	}
}
