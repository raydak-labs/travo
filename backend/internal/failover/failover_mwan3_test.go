package failover

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
	"github.com/openwrt-travel-gui/backend/internal/uci"
)

func TestVerifyApplyRequiresEveryEnabledCandidate(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	svc, _ := newFailoverTestService(t, mockUCI, &platform.NoopUCIApplyConfirm{})

	// usb0 is enabled and available but has no runtime interface, so a config
	// that "works" for wan/wwan must not verify clean.
	cfg := failoverConfigFile{
		Enabled: true,
		Candidates: []models.FailoverCandidate{
			{InterfaceName: "wan", Available: true, Enabled: true, Priority: 1},
			{InterfaceName: "wwan", Available: true, Enabled: true, Priority: 2},
			{InterfaceName: "usb0", Available: true, Enabled: true, Priority: 3},
		},
		Health: defaultFailoverHealth(),
	}
	_ = mockUCI.AddSection("mwan3", "travo_failover", "policy")
	_ = mockUCI.AddSection("mwan3", "travo_default_v4", "rule")

	if err := svc.verifyApply(cfg); err == nil {
		t.Fatal("expected verifyApply to fail while an enabled candidate is missing at runtime")
	}
}

func TestVerifyApplyAcceptsAllEnabledCandidatesPresent(t *testing.T) {
	t.Parallel()

	mockUCI := uci.NewMockUCI()
	svc, _ := newFailoverTestService(t, mockUCI, &platform.NoopUCIApplyConfirm{})
	candidates := []models.FailoverCandidate{
		{InterfaceName: "wan", Available: true, Enabled: true, Priority: 1},
		{InterfaceName: "wwan", Available: true, Enabled: true, Priority: 2},
	}
	seedGeneratedManagedSections(t, mockUCI, candidates)

	cfg := failoverConfigFile{
		Enabled:    true,
		Candidates: candidates,
		Health:     defaultFailoverHealth(),
	}
	if err := svc.verifyApply(cfg); err != nil {
		t.Fatalf("verifyApply: %v", err)
	}
}

// mwan3 members reference the mwan3 INTERFACE SECTION, so a member left
// pointing at a pre-namespacing name ("wan") binds the policy to a section this
// service no longer writes. Presence-only verification passed that in
// production; only the unit test noticed.
func TestVerifyApplyRejectsStaleGeneratedSectionValues(t *testing.T) {
	t.Parallel()

	candidates := []models.FailoverCandidate{
		{InterfaceName: "wan", Available: true, Enabled: true, Priority: 1},
	}
	cfg := failoverConfigFile{
		Enabled:    true,
		Candidates: candidates,
		Health:     defaultFailoverHealth(),
	}

	t.Run("member references a non-namespaced section", func(t *testing.T) {
		t.Parallel()
		mockUCI := uci.NewMockUCI()
		svc, _ := newFailoverTestService(t, mockUCI, &platform.NoopUCIApplyConfirm{})
		seedGeneratedManagedSections(t, mockUCI, candidates)
		if err := mockUCI.Set("mwan3", "travo_wan_p1", "interface", "wan"); err != nil {
			t.Fatalf("Set member interface: %v", err)
		}

		err := svc.verifyApply(cfg)
		if err == nil {
			t.Fatal("verifyApply accepted a member pointing at a stale section name")
		}
		if !strings.Contains(err.Error(), "travo_wan_p1") {
			t.Errorf("the failure must name the offending member, got %v", err)
		}
	})

	t.Run("generated interface section lost its family", func(t *testing.T) {
		t.Parallel()
		mockUCI := uci.NewMockUCI()
		svc, _ := newFailoverTestService(t, mockUCI, &platform.NoopUCIApplyConfirm{})
		seedGeneratedManagedSections(t, mockUCI, candidates)
		if err := mockUCI.DeleteOption("mwan3", failoverIfWan, "family"); err != nil {
			t.Fatalf("DeleteOption: %v", err)
		}

		err := svc.verifyApply(cfg)
		if err == nil {
			t.Fatal("verifyApply accepted a generated interface section without family")
		}
		if !strings.Contains(err.Error(), "family") {
			t.Errorf("the failure must name the offending option, got %v", err)
		}
	})
}

// Ownership is proven by provenance only: the fingerprint THIS build wrote, on
// a namespaced section. Content proves nothing — a non-namespaced section whose
// options are exactly what a save writes is still the operator's copy of the
// stock mwan3 example.
func TestMayDeleteSectionRecognisesOnlyTravoSections(t *testing.T) {
	t.Parallel()

	owned := map[string]string{"enabled": "0"}
	owned[failoverOwnerOption] = sectionFingerprint(owned)
	edited := map[string]string{"enabled": "0", "check_quality": "1"}
	edited[failoverOwnerOption] = owned[failoverOwnerOption]

	// Byte-identical to what this service writes for a default-health enabled
	// wan candidate, and carrying no fingerprint: not provable, so not ours.
	candidate := models.FailoverCandidate{
		InterfaceName: "wan", Kind: models.FailoverCandidateKindEthernet,
		Available: true, Enabled: true, Priority: 1,
	}
	indistinguishable := map[string]string{".type": "interface"}
	for option, value := range interfaceSectionOptions(candidate, failoverTestHealth()) {
		indistinguishable[option] = value
	}
	indistinguishable["track_ip"] = "1.1.1.1 8.8.8.8"

	cases := []struct {
		name string
		opts map[string]string
		want bool
	}{
		{"travo_if_wan", owned, true},
		{"travo_if_wan", edited, false}, // operator edited it after Travo wrote it
		{"travo_if_wan", map[string]string{".type": "interface"}, false},
		{"hotel", map[string]string{".type": "interface", "ifname": "eth3"}, false},
		{"my_custom_policy", map[string]string{".type": "policy"}, false},
		// No fingerprint, no provenance: never ours, however exact the content.
		{"wan", indistinguishable, false},
		{"usb0", indistinguishable, false},
	}
	for _, tc := range cases {
		if got := mayDeleteSection(tc.name, tc.opts); got != tc.want {
			t.Errorf("mayDeleteSection(%q, %v) = %v, want %v", tc.name, tc.opts, got, tc.want)
		}
	}
}

// The backup predicate is deliberately WIDER than the deletion predicate: a save
// writes over every namespaced section it regenerates, so every one of them must
// be restorable — including an operator-edited one that must not be deleted.
func TestNeedsBackupSectionCoversEveryNamespacedSection(t *testing.T) {
	t.Parallel()

	owned := map[string]string{"enabled": "0"}
	owned[failoverOwnerOption] = sectionFingerprint(owned)
	edited := map[string]string{"enabled": "0", "check_quality": "1"}
	edited[failoverOwnerOption] = owned[failoverOwnerOption]

	cases := []struct {
		name       string
		opts       map[string]string
		wantDelete bool
		wantBackup bool
	}{
		{"travo_if_wan", owned, true, true},
		{"travo_if_wan", edited, false, true}, // adopted, but still overwritten
		{"travo_failover", edited, false, true},
		{"travo_default_v4", edited, false, true},
		{"hotel", map[string]string{".type": "interface"}, false, false},
		{"wan", map[string]string{".type": "interface"}, false, false},
	}
	for _, tc := range cases {
		if got := mayDeleteSection(tc.name, tc.opts); got != tc.wantDelete {
			t.Errorf("mayDeleteSection(%q) = %v, want %v", tc.name, got, tc.wantDelete)
		}
		if got := needsBackupSection(tc.name); got != tc.wantBackup {
			t.Errorf("needsBackupSection(%q) = %v, want %v", tc.name, got, tc.wantBackup)
		}
	}
}

// rpcd allows only one pending rollback at a time and rejects a second
// rollback-enabled apply while the first is unconfirmed. After a failed verify
// the window is still open, so the next apply — including a restore attempt
// inside the same save — must not be issued. It must also be recognised by the
// next save rather than surfacing as an opaque "permission denied" from rpcd.
func TestStagedApplyMwan3_SkipsWhileARollbackIsPending(t *testing.T) {
	applier := &recordingApplier{}
	svc, _ := newFailoverTestService(t, uci.NewMockUCI(), applier)

	// First call: StartApply succeeds, verification fails, window stays open.
	if err := svc.stagedApplyMwan3(func() error { return fmt.Errorf("verification failed") }); err == nil {
		t.Fatal("expected the failed verification to be reported")
	}
	if svc.pendingApplySession == "" {
		t.Fatal("an unconfirmed apply must stay recorded while its rollback window is open")
	}
	first := len(applier.startCalls)

	// Second call while that window is open: refused, not re-issued, and
	// reported as a failure. Returning nil here acknowledged a save that rpcd
	// was about to revert, so the caller dropped its crash guard and answered
	// 200 for a change that never became live.
	if err := svc.stagedApplyMwan3(nil); !errors.Is(err, errApplyRollingBack) {
		t.Fatalf("a save inside a pending rollback window must fail with errApplyRollingBack, got %v", err)
	}
	if len(applier.startCalls) != first {
		t.Errorf("a second StartApply was issued while a rollback was pending (%d -> %d)", first, len(applier.startCalls))
	}
}

// Once the pending apply is confirmed the window is closed and applies resume.
func TestStagedApplyMwan3_ResumesAfterConfirm(t *testing.T) {
	applier := &recordingApplier{}
	svc, _ := newFailoverTestService(t, uci.NewMockUCI(), applier)

	if err := svc.stagedApplyMwan3(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.pendingApplySession != "" {
		t.Errorf("a confirmed apply must not stay pending, got %q", svc.pendingApplySession)
	}
	first := len(applier.startCalls)
	if err := svc.stagedApplyMwan3(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(applier.startCalls) != first+1 {
		t.Errorf("applies should resume after a confirm (%d -> %d)", first, len(applier.startCalls))
	}
}

// rpcd rolls a failed apply back on its own timer and never tells us, so the
// record of the unconfirmed session has to expire on its own. Otherwise it
// survives the rollback and every later failover apply is skipped for the life
// of the process.
func TestStagedApplyMwan3_ResumesAfterTheRollbackWindowExpires(t *testing.T) {
	applier := &recordingApplier{}
	svc, _ := newFailoverTestService(t, uci.NewMockUCI(), applier)

	if err := svc.stagedApplyMwan3(func() error { return fmt.Errorf("verification failed") }); err == nil {
		t.Fatal("expected the failed verification to be reported")
	}
	if svc.pendingApplySession == "" {
		t.Fatal("an unconfirmed apply must stay recorded while its rollback window is open")
	}
	first := len(applier.startCalls)

	// Still inside the window: refused.
	if err := svc.stagedApplyMwan3(nil); !errors.Is(err, errApplyRollingBack) {
		t.Fatalf("a save inside the rollback window must report errApplyRollingBack, got %v", err)
	}
	if len(applier.startCalls) != first {
		t.Fatalf("an apply was issued while the rollback window was still open")
	}

	// Once the window has passed, applies must resume.
	svc.pendingApplyDeadline = time.Now().Add(-time.Second)
	if err := svc.stagedApplyMwan3(nil); err != nil {
		t.Fatalf("unexpected error after the rollback window: %v", err)
	}
	if len(applier.startCalls) != first+1 {
		t.Errorf("applies did not resume after the rollback window expired (%d -> %d)", first, len(applier.startCalls))
	}
}
