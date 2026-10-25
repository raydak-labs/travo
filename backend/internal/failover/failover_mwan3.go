package failover

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/openwrt-travel-gui/backend/internal/models"
	"github.com/openwrt-travel-gui/backend/internal/platform"
)

// mayDeleteSection reports whether a save may DELETE an mwan3 section. This is
// the only question the ownership fingerprint answers, and it is asked only here.
//
// Ownership is PROVEN BY PROVENANCE, never inferred from content. A section is
// Travo's if and only if it carries the `travo_owner` fingerprint THIS build
// wrote and that fingerprint still matches what it wrote. Deleting is the one
// thing this service cannot undo: the backup the section would have gone into is
// only read when a save FAILS, and nothing puts a section back after a
// successful one — a dropped config section is lost, and mwan3 stops tracking
// that uplink permanently.
//
// Content is therefore never a basis for deletion, and the reason is concrete:
// the stock mwan3 example an operator copies is byte-identical to what this
// service writes for a default-health candidate, so an exact option-set match
// cannot tell the operator's copy from Travo's own output. A non-namespaced
// section is never deleted, whatever it contains (ADR 0005 §1).
func mayDeleteSection(name string, opts map[string]string) bool {
	return strings.HasPrefix(name, failoverSectionPrefix) && ownedByTravo(opts)
}

// needsBackupSection reports whether a save may WRITE to an mwan3 section, and
// so must be able to restore its pre-save state. It is deliberately WIDER than
// mayDeleteSection, because the two answer different questions:
//
//   - "may I delete this?"  → provenance. Provenance is the right answer, and
//     the only safe one: an operator-edited section is kept.
//   - "can I put this back if the apply fails?" → reach. A save regenerates
//     EVERY namespaced section it owns a name for, and regenerating means
//     `uci set` over all of its options, so the pre-save state of an
//     operator-edited travo_ section is destroyed too. Sharing one predicate
//     here left that state in no backup at all, on a path that is only ever read
//     when the save FAILED — the one moment the operator's tuning mattered. The
//     section survived the delete pass and was overwritten anyway.
//
// Adopting a section is therefore not "leave it alone": it means "keep it, and
// be able to roll it back".
func needsBackupSection(name string) bool {
	return strings.HasPrefix(name, failoverSectionPrefix)
}

// ownedByTravo reports whether a namespaced section still carries exactly what
// this service wrote to it.
func ownedByTravo(opts map[string]string) bool {
	marker := strings.TrimSpace(opts[failoverOwnerOption])
	return marker != "" && marker == sectionFingerprint(opts)
}

// normaliseOptionValue reduces a UCI value to comparable tokens. GetAll and
// GetSections do not agree on list formatting — one reports "a b", the other
// "a,b" — so every comparison and fingerprint goes through this, or a section
// this service just wrote would read back as user-modified.
func normaliseOptionValue(value string) string {
	tokens := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
	slices.Sort(tokens)
	tokens = slices.Compact(tokens)
	return strings.Join(tokens, " ")
}

// failoverOwnerOption marks a section as this service's, and records what it
// wrote so an operator edit is detectable. Section metadata is excluded: GetAll
// drops it and GetSections keeps it, so including it would make every section
// look modified the moment the two are compared.
const failoverOwnerOption = "travo_owner"

// sectionFingerprint is a short digest of a section's options, ignoring the
// marker itself.
func sectionFingerprint(opts map[string]string) string {
	keys := make([]string, 0, len(opts))
	for key := range opts {
		if key == failoverOwnerOption {
			continue
		}
		keys = append(keys, key)
	}
	slices.Sort(keys)
	digest := sha256.New()
	for _, key := range keys {
		_, _ = fmt.Fprintf(digest, "%s=%s\n", key, normaliseOptionValue(opts[key]))
	}
	return hex.EncodeToString(digest.Sum(nil))[:16]
}

// markSectionOwned stamps the ownership marker. It runs after every other
// option of the section is written, because the fingerprint covers them all.
func (s *FailoverService) markSectionOwned(section string) error {
	opts, err := s.uci.GetAll(mwan3ConfigName, section)
	if err != nil {
		return fmt.Errorf("read generated mwan3 section %s: %w", section, err)
	}
	if err := s.uci.Set(mwan3ConfigName, section, failoverOwnerOption, sectionFingerprint(opts)); err != nil {
		return fmt.Errorf("marking mwan3 section %s as owned: %w", section, err)
	}
	return nil
}

// interfaceSectionOptions is the option map writeInterfaceSection produces.
// track_ip is deliberately absent: it is a list this service appends to
// separately, and emitting it here as a joined (or empty) string is how an
// option that does not exist and one that exists compare as different.
func interfaceSectionOptions(candidate models.FailoverCandidate,
	health models.FailoverHealthConfig,
) map[string]string {
	return map[string]string{
		"enabled":           boolToUCI(candidate.Enabled),
		"family":            "ipv4",
		"reliability":       fmt.Sprintf("%d", health.Reliability),
		"count":             fmt.Sprintf("%d", health.Count),
		"timeout":           fmt.Sprintf("%d", health.Timeout),
		"interval":          fmt.Sprintf("%d", health.Interval),
		"failure_interval":  fmt.Sprintf("%d", health.FailureInterval),
		"recovery_interval": fmt.Sprintf("%d", health.RecoveryInterval),
		"down":              fmt.Sprintf("%d", health.Down),
		"up":                fmt.Sprintf("%d", health.Up),
	}
}

// operatorEditsOverwrittenAlert is the alert type raised when a save replaces
// options an operator had hand-tuned inside a travo_ section.
const operatorEditsOverwrittenAlert = "failover_operator_edits_overwritten"

// failoverSectionPrefix namespaces every mwan3 section this service writes.
const failoverSectionPrefix = "travo_"

// failoverInterfaceSectionPrefix prefixes a generated mwan3 interface section.
// mwan3 tracks by the mwan3 section name, so members reference THIS name, not
// the network interface name.
const failoverInterfaceSectionPrefix = failoverSectionPrefix + "if_"

// failoverInterfaceSection is the mwan3 interface section name for a candidate.
func failoverInterfaceSection(interfaceName string) string {
	return failoverInterfaceSectionPrefix + failoverSectionName(interfaceName)
}

func (s *FailoverService) backupManagedSections() error {
	sections, err := s.uci.GetSections(mwan3ConfigName)
	if err != nil {
		sections = map[string]map[string]string{}
	}
	managed := map[string]map[string]string{}
	for name, opts := range sections {
		// Backup by section shape, not by the new candidate list: a section left
		// over from a removed candidate must still be restorable — and so must
		// an operator-edited one the save is about to overwrite (needsBackupSection,
		// which is wider than mayDeleteSection on purpose).
		if needsBackupSection(name) {
			managed[name] = opts
		}
	}
	data, err := json.Marshal(managed)
	if err != nil {
		return fmt.Errorf("marshal failover backup: %w", err)
	}
	// Atomic write: restoreManagedSections reads this on the rollback path, and
	// a truncated backup makes the restore fail after the failure it was
	// supposed to repair.
	return platform.WriteFileAtomic(s.backupPath, data)
}

// restoreManagedSections is the rollback path: it puts the backed-up
// mwan3/network sections back, unchanged, and re-applies them, so the running
// mwan3 does not keep a broken partial policy while the user is told the save
// failed.
//
// The re-apply deliberately BYPASSES the pending-rollback-window check that
// stagedApplyMwan3 enforces: it runs precisely while a session is pending, and
// short-circuiting on errApplyRollingBack would leave the crash guard behind
// after every failed save.
func (s *FailoverService) restoreManagedSections() error {
	data, err := os.ReadFile(s.backupPath)
	if err != nil {
		return err
	}
	var sections map[string]map[string]string
	if err := json.Unmarshal(data, &sections); err != nil {
		return err
	}
	// Delete under the same provenance rule the save used, so a failed save
	// unwinds exactly what it changed and nothing else.
	if err := s.deleteManagedSections(); err != nil {
		return err
	}
	for name, opts := range sections {
		stype := opts[".type"]
		if stype == "" {
			continue
		}
		if err := s.uci.AddSection(mwan3ConfigName, name, stype); err != nil {
			return fmt.Errorf("restore add section %s: %w", name, err)
		}
		for option, value := range opts {
			if strings.HasPrefix(option, ".") {
				continue
			}
			if mwan3ListOptions[option] {
				if err := s.restoreListOption(name, option, value); err != nil {
					return err
				}
				continue
			}
			if strings.Contains(value, ",") && !mwan3ScalarMayContainComma[option] {
				slog.Warn("failover: restoring a joined value as a scalar; add the option to mwan3ListOptions if it is a list", "section", name, "option", option, "value", value)
			}
			if err := s.uci.Set(mwan3ConfigName, name, option, value); err != nil {
				return err
			}
		}
	}
	if err := s.uci.Commit(mwan3ConfigName); err != nil {
		return err
	}
	return s.restoreApplyMwan3(func() error { return s.verifyManagedSections(sections) })
}

func (s *FailoverService) restoreListOption(section, option, value string) error {
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if err := s.uci.AddList(mwan3ConfigName, section, option, item); err != nil {
			return fmt.Errorf("restore list %s.%s: %w", section, option, err)
		}
	}
	return nil
}

func (s *FailoverService) deleteManagedSections() error {
	sections, err := s.uci.GetSections(mwan3ConfigName)
	if err != nil {
		return nil
	}
	for name, opts := range sections {
		if mayDeleteSection(name, opts) {
			_ = s.uci.DeleteSection(mwan3ConfigName, name)
		}
	}
	return s.uci.Commit(mwan3ConfigName)
}

func (s *FailoverService) applyManagedConfig(cfg failoverConfigFile) error {
	if err := s.deleteManagedSections(); err != nil {
		return err
	}
	if !cfg.Enabled || !s.serviceInstalled() {
		return s.stagedApplyMwan3(nil)
	}

	for _, candidate := range cfg.Candidates {
		if err := s.writeInterfaceSection(candidate, cfg.Health); err != nil {
			return err
		}
		if candidate.Enabled {
			memberName := fmt.Sprintf("travo_%s_p%d", failoverSectionName(candidate.InterfaceName), candidate.Priority)
			if err := s.adoptSection(memberName, "member"); err != nil {
				return err
			}
			// mwan3 members reference the mwan3 INTERFACE SECTION name, which
			// is the namespaced one, not the network interface.
			iface := failoverInterfaceSection(candidate.InterfaceName)
			if err := s.uci.Set(mwan3ConfigName, memberName, "interface", iface); err != nil {
				return err
			}
			if err := s.uci.Set(mwan3ConfigName, memberName, "metric", fmt.Sprintf("%d", candidate.Priority)); err != nil {
				return err
			}
			if err := s.uci.Set(mwan3ConfigName, memberName, "weight", "1"); err != nil {
				return err
			}
			if err := s.markSectionOwned(memberName); err != nil {
				return err
			}
		}
	}
	if err := s.adoptSection(failoverPolicySection, "policy"); err != nil {
		return err
	}
	for _, candidate := range cfg.Candidates {
		if !candidate.Enabled {
			continue
		}
		memberName := fmt.Sprintf("travo_%s_p%d", failoverSectionName(candidate.InterfaceName), candidate.Priority)
		if err := s.uci.AddList(mwan3ConfigName, failoverPolicySection, "use_member", memberName); err != nil {
			return err
		}
	}
	if err := s.adoptSection(failoverRuleSection, "rule"); err != nil {
		return err
	}
	if err := s.uci.Set(mwan3ConfigName, failoverRuleSection, "dest_ip", "0.0.0.0/0"); err != nil {
		return fmt.Errorf("set rule dest_ip: %w", err)
	}
	if err := s.uci.Set(mwan3ConfigName, failoverRuleSection, "family", "ipv4"); err != nil {
		return fmt.Errorf("set rule family: %w", err)
	}
	if err := s.uci.Set(mwan3ConfigName, failoverRuleSection, "use_policy", failoverPolicySection); err != nil {
		return fmt.Errorf("set rule use_policy: %w", err)
	}
	if err := s.markSectionOwned(failoverPolicySection); err != nil {
		return err
	}
	if err := s.markSectionOwned(failoverRuleSection); err != nil {
		return err
	}
	if err := s.uci.Commit(mwan3ConfigName); err != nil {
		return err
	}
	return s.stagedApplyMwan3(func() error { return s.verifyManagedSections(managedSectionNames(cfg.Candidates)) })
}

// adoptSection creates or reuses an mwan3 section for writing. Real
// `uci set config.section=stype` is create-OR-update, so a section that survived
// the delete pass is reused here and every option below is written over it.
//
// The one case worth telling the user about is reuse of an operator-edited
// section: needsBackupSection says the save may write it, mayDeleteSection says
// the save may not delete it, so the operator's extra options and their section
// type survive while the options this service owns are replaced by the saved
// failover settings. That is the deliberate outcome (ADR 0005 §1.2) — Travo's
// saved health config is authoritative for the options it writes, and
// verifyManagedSections requires the live config to match it exactly — but
// reporting it is the difference between a documented take-over and a silent
// loss of the operator's reliability/timeout/up/down tuning.
func (s *FailoverService) adoptSection(section, stype string) error {
	opts, err := s.uci.GetAll(mwan3ConfigName, section)
	if err == nil && needsBackupSection(section) && !ownedByTravo(opts) {
		slog.Warn("failover: mwan3 section edited outside Travo; this save replaces the options Travo owns", "section", section)
		if s.alertSvc != nil {
			s.alertSvc.Publish(operatorEditsOverwrittenAlert, fmt.Sprintf(
				"Failover adopted mwan3 section %s: options Travo owns were replaced by the saved "+
					"failover settings, other options were kept. Re-apply your tuning there if it was lost.",
				section), "warning")
		}
	}
	return s.uci.AddSection(mwan3ConfigName, section, stype)
}

func (s *FailoverService) writeInterfaceSection(candidate models.FailoverCandidate, health models.FailoverHealthConfig) error {
	// Namespaced (ADR 0005 §1): a generated section that shares its name with a
	// hand-written one is indistinguishable from it on the next save, and the
	// next save deletes it.
	sectionName := failoverInterfaceSection(candidate.InterfaceName)
	if err := s.adoptSection(sectionName, "interface"); err != nil {
		return err
	}
	for option, value := range interfaceSectionOptions(candidate, health) {
		if option == "track_ip" {
			continue
		}
		if err := s.uci.Set(mwan3ConfigName, sectionName, option, value); err != nil {
			return fmt.Errorf("set interface %s: %w", option, err)
		}
	}
	for _, ip := range health.TrackIPs {
		if err := s.uci.AddList(mwan3ConfigName, sectionName, "track_ip", ip); err != nil {
			if setErr := s.uci.Set(mwan3ConfigName, sectionName, "track_ip", ip); setErr != nil {
				return fmt.Errorf("set track_ip %s: %w", ip, setErr)
			}
		}
	}
	return s.markSectionOwned(sectionName)
}

func (s *FailoverService) verifyApply(cfg failoverConfigFile) error {
	var expect map[string]map[string]string
	if cfg.Enabled {
		// Every generated section, with the options that bind them together —
		// not just the policy and the rule.
		expect = managedSectionNames(cfg.Candidates)
	}
	if err := s.verifyManagedSections(expect); err != nil {
		return err
	}
	networkStatus, err := s.networkSvc.GetNetworkStatus()
	if err != nil {
		return err
	}
	// Every enabled, available candidate must be readable at runtime. Requiring
	// only one of them would let a misconfigured uplink verify clean while the
	// others work, and the user would only find out when that link is needed.
	var expected []string
	for _, candidate := range cfg.Candidates {
		if candidate.Enabled && candidate.Available {
			expected = append(expected, candidate.InterfaceName)
		}
	}
	for _, name := range expected {
		if !interfacePresent(networkStatus.Interfaces, name) {
			return fmt.Errorf("enabled failover candidate %s is not readable at runtime", name)
		}
	}
	return nil
}

// verifyManagedSections checks that the mwan3 config the service wrote is
// readable. When expect is non-nil, the given sections must be present and
// carry the option values in the map (an empty value is not checked).
func (s *FailoverService) verifyManagedSections(expect map[string]map[string]string) error {
	sections, err := s.uci.GetSections(mwan3ConfigName)
	if err != nil {
		return fmt.Errorf("verify mwan3 config: %w", err)
	}
	for name, opts := range expect {
		actual, ok := sections[name]
		if !ok {
			return fmt.Errorf("mwan3 section %s missing after apply", name)
		}
		for option, want := range opts {
			// Section metadata (".type") is not an option.
			if want == "" || strings.HasPrefix(option, ".") {
				continue
			}
			// Compare against the same read the expectation was built from:
			// GetSections and Get normalise list values differently, so mixing
			// them would fail the rollback restore of any section with a list.
			if actual[option] != want {
				return fmt.Errorf("mwan3 section %s: %s = %q, want %q", name, option, actual[option], want)
			}
		}
	}
	return nil
}

var mwan3UCIConfigs = []string{"network", "mwan3"}

// stagedApplyMwan3 applies the mwan3 config for a user save, refusing to start
// while an earlier apply is still rolling back.
func (s *FailoverService) stagedApplyMwan3(verify func() error) error {
	if s.rollbackStillPending() {
		// An earlier apply in this service is unconfirmed, so its rollback window
		// is open and rpcd is restoring the pre-apply mwan3 config right now. A
		// second apply would be refused, and reporting success would be worse: the
		// caller deleted the guard and answered 200 while rpcd's timer was about
		// to restore the config from *before* the earlier save — silently
		// discarding both the failed config and this acknowledged one.
		slog.Info("failover: an mwan3 apply is already rolling back; skipping this apply", "session", s.pendingApplySession)
		return errApplyRollingBack
	}
	// The window expired and rpcd rolled the session back; nothing confirms
	// that, so the deadline is the signal. Forget it and apply.
	s.pendingApplySession = ""
	return s.applyMwan3(verify)
}

// restoreApplyMwan3 is the recovery-path variant: it runs even while an earlier
// apply is still rolling back, because restoring is exactly what must be able to
// happen then. The stale session id is dropped first — rpcd has a single apply
// slot, so carrying it into the restore's own apply would record the restore's
// session under the old deadline if the restore then failed.
func (s *FailoverService) restoreApplyMwan3(verify func() error) error {
	s.pendingApplySession = ""
	return s.applyMwan3(verify)
}

// applyMwan3 runs the rpcd apply+confirm flow with a real rollback window: the
// apply is started first, the config is verified while the previous config is
// still restorable, and only then is the change confirmed. An immediate
// apply+confirm on a policy that re-routes the WAN would close the rollback
// window before anything was checked.
func (s *FailoverService) applyMwan3(verify func() error) error {
	if !s.serviceInstalled() {
		return nil
	}
	if s.applier == nil {
		return s.reloadMwan3Script()
	}
	sid, err := s.applier.StartApply(mwan3UCIConfigs)
	if err != nil {
		return fmt.Errorf("uci apply mwan3: %w", err)
	}
	if sid == "" {
		// Applier had nothing to do (Noop / empty session).
		return nil
	}
	s.pendingApplySession = sid
	// rpcd rolls back after its own timeout; add a margin so the record is only
	// dropped once the rollback has certainly happened.
	s.pendingApplyDeadline = time.Now().Add(rollbackGracePeriod)
	if verify != nil {
		if err := verify(); err != nil {
			// Rollback window is still open: rpcd reverts to the previous config
			// on its own timer. Keep the session recorded until that deadline so
			// the next apply waits for the rollback instead of being rejected.
			return fmt.Errorf("verify mwan3 apply: %w", err)
		}
	}
	if err := s.applier.Confirm(sid); err != nil {
		return fmt.Errorf("uci confirm mwan3: %w", err)
	}
	s.pendingApplySession = ""
	return nil
}

// reloadMwan3Script is the fallback for services without an rpcd applier.
func (s *FailoverService) reloadMwan3Script() error {
	if _, err := s.cmd.Run(s.initScript, "reload"); err != nil {
		if _, restartErr := s.cmd.Run(s.initScript, "restart"); restartErr != nil {
			return fmt.Errorf("reload mwan3: %w", err)
		}
	}
	return nil
}

func (s *FailoverService) serviceInstalled() bool {
	_, err := os.Stat(s.initScript)
	return err == nil
}
