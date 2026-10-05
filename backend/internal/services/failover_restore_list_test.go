package services

import (
	"encoding/json"
	"os"
	"path/filepath"

	"testing"

	"github.com/openwrt-travel-gui/backend/internal/ubus"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

// The device artefact this reproduces: after a failed save, /etc/config/mwan3
// carried
//
//	travo_if_wan.track_ip='1.1.1.1,8.8.8.8' '1.1.1.1' '8.8.8.8'
//	travo_failover.use_member='travo_wan_p1,travo_wwan_p2' 'travo_wan_p1' 'travo_wwan_p2'
//
// The first element is one comma-joined string, which mwan3 reads as a single
// (invalid) address. A fresh save writes the correct list; only the restore path
// produces this, because RealUCI.GetAll normalises a printed list
// ('a' 'b') into the comma-joined form a,b so callers can split on it, and the
// restore writes that value back with Set.
func newRestoreTestService(t *testing.T) (*FailoverService, *uci.MockUCI) {
	t.Helper()
	u := uci.NewMockUCI()
	mockUbus := ubus.NewMockUbus()
	networkSvc := NewNetworkServiceWithRunner(u, mockUbus, &MockCommandRunner{})
	svc := NewFailoverServiceWithRunner(u, mockUbus, networkSvc,
		&MockCommandRunner{}, &NoopUCIApplyConfirm{},
		filepath.Join(t.TempDir(), "failover.json"))
	svc.backupPath = filepath.Join(t.TempDir(), "mwan3-backup.json")
	return svc, u
}

func TestRestoreManagedSectionsRebuildsListOptionsAsLists(t *testing.T) {
	svc, u := newRestoreTestService(t)

	// What backupManagedSections persists, read back through RealUCI's
	// normalising GetAll: list options arrive comma-joined.
	backup := map[string]map[string]string{
		"travo_if_wan": {
			".type":    "interface",
			"enabled":  "1",
			"track_ip": "1.1.1.1,8.8.8.8",
		},
		"travo_failover": {
			".type":      "policy",
			"use_member": "travo_wan_p1,travo_wwan_p2",
		},
	}
	writeBackup(t, svc.backupPath, backup)

	if err := svc.restoreManagedSections(); err != nil {
		t.Fatalf("restoreManagedSections: %v", err)
	}

	track, err := u.GetAll("mwan3", "travo_if_wan")
	if err != nil {
		t.Fatalf("read restored interface: %v", err)
	}
	// MockUCI stores a list as its elements space-joined, and GetAll returns
	// that verbatim. RealUCI.GetAll instead normalises a printed list to the
	// comma-joined form, which is what lands in the backup. So a COMMA in the
	// value here means "one element that came from a comma-join", which is
	// exactly the corruption being pinned — asserting on a comma-split would
	// hide it.
	if got := track["track_ip"]; got != "1.1.1.1 8.8.8.8" {
		t.Fatalf("track_ip must be restored as two list elements, got %q", got)
	}

	policy, err := u.GetAll("mwan3", "travo_failover")
	if err != nil {
		t.Fatalf("read restored policy: %v", err)
	}
	if got := policy["use_member"]; got != "travo_wan_p1 travo_wwan_p2" {
		t.Fatalf("use_member must be restored as two list elements, got %q", got)
	}
}

// A list option Travo does not know about must not be silently written as one
// joined element. The restore logs it loudly rather than corrupting it quietly,
// because a corrupted health-tracking address is invisible until failover stops
// working.
func TestRestoreManagedSectionsWarnsOnAnUnknownListShapedOption(t *testing.T) {
	svc, u := newRestoreTestService(t)

	writeBackup(t, svc.backupPath, map[string]map[string]string{
		"travo_if_wan": {
			".type":      "interface",
			"enabled":    "1",
			"custom_ips": "10.0.0.1,10.0.0.2",
			"family":     "ipv4",
		},
	})

	if err := svc.restoreManagedSections(); err != nil {
		t.Fatalf("restoreManagedSections: %v", err)
	}
	opts, err := u.GetAll("mwan3", "travo_if_wan")
	if err != nil {
		t.Fatalf("read restored interface: %v", err)
	}
	if opts["custom_ips"] == "" {
		t.Fatal("an unknown option must still be restored, not dropped")
	}
	if opts["family"] != "ipv4" {
		t.Fatalf("a scalar option must be restored unchanged, got %q", opts["family"])
	}
}

func writeBackup(t *testing.T, path string, sections map[string]map[string]string) {
	t.Helper()
	data, err := json.Marshal(sections)
	if err != nil {
		t.Fatalf("marshal backup: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write backup: %v", err)
	}
}
