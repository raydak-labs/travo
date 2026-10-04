---
title: "ADR 0005: Multi-WAN connection failover (mwan3)"
status: Accepted
date: 2026-05-14
updated: 2026-10-04
tags: [adr, mwan3, failover, routing, ipv4]
---

# ADR 0005: Multi-WAN connection failover (mwan3)

## Status

Accepted.

## Context

Travo implements **priority-based WAN failover** using OpenWrt’s **`mwan3`** package: health checks, policy routing, and interface tracking. Policy must be **deterministic**, **namespaced** to avoid colliding with user hand-edits, and **safe to apply** under crash-guard rules. IPv6 behavior on mwan3 across target OpenWrt versions was uncertain at productization time.

## Decision

### 1. Source of truth and generation

- Operator-facing failover configuration is stored in **`/etc/travo/failover.json`**.
- Travo **generates** UCI sections into **`/etc/config/mwan3`** (and coordinates **`network`** where required) from that JSON via **`FailoverService`**.
- App-owned generated sections use a **`travo_`** name prefix to reduce collision with manual mwan3 config.
- **Ownership is PROVEN BY PROVENANCE, never inferred from content.** A section is Travo's **if and only if** it carries the **`travo_owner`** fingerprint *this build* wrote and that fingerprint still matches the section's current options.
- **Why content cannot decide this (2026-10-04, deliberate reversal).** An earlier revision treated a non-namespaced section as Travo's when its name matched a configured candidate *and* its option set was **exactly** what the save writes. That rule was executed against the stock OpenWrt `/etc/config/mwan3` example and deleted the operator's `config interface 'wan'`: the stock example is byte-identical to what this service writes for a default-health candidate, which is precisely why exact matching cannot help. Content-based inference has no discriminating power here — the two configurations are the same bytes.
- The delete is **irreversible in practice**: the backup is only read when a save *fails*, so nothing restores a section deleted on the success path. Per this service's own note, mwan3 then stops tracking that uplink permanently. Therefore:
  - **A non-namespaced section is never deleted**, whatever it contains — an identical copy of our own output included. Preferring a visible leftover an operator can inspect and remove with `uci delete mwan3.<section>` over deleting a config Travo cannot prove it owns.
  - **A `travo_` section the operator edited** no longer matches its fingerprint and is **adopted and preserved**: never deleted, and regenerated in place (the operator's extra options survive) rather than treated as a conflict. See §1.2 for what "regenerated in place" costs the operator, and §1.3 for why preservation and restorability are two different predicates.

### 1.2 What a save writes to an adopted (operator-edited) section

- **The write applies Travo's values to the options Travo owns.** `writeInterfaceSection` reuses the section (`uci set config.section=stype` is create-**or**-update) and re-`Set`s every option in `interfaceSectionOptions` — `enabled`, `family`, `reliability`, `count`, `timeout`, `interval`, `failure_interval`, `recovery_interval`, `down`, `up` — then re-stamps `travo_owner`. The operator's **additional** options (e.g. `check_quality`) and their section type survive; their **tuning of Travo's options does not**.
- **This is deliberate, and it is a product decision, not an implementation accident.** The saved failover health config is the authoritative source for those options, and `verifyManagedSections` requires the live mwan3 config to match it **exactly** — a section keeping hand-tuned values could not verify, so every later save would fail. Preserving the operator's values instead would mean the UI's health settings silently stop applying to that candidate, which is a worse surprise than an over-write that is announced.
- **Therefore the over-write must never be silent.** `adoptSection` detects the reuse of a namespaced section whose fingerprint no longer matches, logs it, and publishes a `failover_operator_edits_overwritten` warning alert naming the section. "Adopted" means *kept and rolled back*, never *left untouched*.

### 1.3 Two predicates: deletable vs. restorable

- **`mayDeleteSection(name, opts)` — provenance.** Namespaced **and** the fingerprint still matches. Used **only** by the delete pass and the rollback's delete pass. An operator-edited section fails this test and is never deleted.
- **`needsBackupSection(name, opts)` — reach.** Namespaced, full stop. Used by `backupManagedSections`. It is deliberately **wider** than the delete predicate, because the two answer different questions:
  - *May I delete this?* → provenance.
  - *Can I put this back if the apply fails?* → whether the save is about to write to it at all.
- **Why one predicate cannot serve both (2026-10-04).** Sharing `isManagedSection` meant an operator-edited `travo_if_wwan` was never deleted **and never backed up**, while `writeInterfaceSection` overwrote all ten of its options and finished with `markSectionOwned`. The sequence — operator hand-tunes `travo_if_wwan` in LuCI → an unrelated failover change is saved → the section survives the delete pass → its reliability/timeout/up/down are replaced by Travo's values → the pre-save state was in no backup, and the backup is only read on the **failure** path → the section is Travo-owned again, so the **next** save deletes it outright. Executed probe: the failed save deleted `travo_if_wwan` completely, because the failed save had re-stamped the fingerprint and the restore's delete pass then owned it, while the backup held nothing to put back.
- **Rule: back up anything the save is about to write.** Restorability is about blast radius, not ownership.
- **Deliberate loss of legacy cleanup.** A pre-namespacing section (named after a candidate, with no fingerprint) is now **never cleaned up**. The upgrade path no longer removes it. That is inert config, not amnesia: `mwan3` keeps tracking it until an operator deletes it, and Travo no longer claims it in its backup. Preserving a leftover the operator can see and delete is strictly safer than deleting a config Travo cannot prove it owns.

### 2. IPv4-only phase

- Phase 1 emits mwan3 rules with **`family: ipv4`** only. **IPv6 failover is explicitly deferred** until validated on hardware (see `docs/plans/connection-failover.md` and `docs/architecture.md` §6.1).

### 3. Safety guards and backup

- Applying failover writes **`/etc/trafo/failover-in-progress`** before live policy mutation and removes it only after successful verification (ADR 0003).
- A backup artifact **`/etc/travo/failover-mwan3-backup.json`** supports restore semantics implemented in `FailoverService` (paired with UCI operations). It captures **every namespaced section the save may write** (§1.3), including operator-edited ones, so a failed apply can hand the tuning back. `restoreManagedSections` restores the backup under the **delete** predicate, so a failed save unwinds exactly what it changed and nothing else.

### 4. Apply path

- Failover uses **`UCIApplyConfirm`** for **`network`** and **`mwan3`** configs (`mwan3UCIConfigs`) where the real applier is wired.
- The apply is **staged**, not immediate: `stagedApplyMwan3` starts the rpcd apply with `rollback: true`, **verifies every managed section** while the previous config is still restorable, and only then calls `Confirm`. Confirming straight after the apply would close the rollback window before anything was checked on a policy that re-routes the WAN. When no applier is wired (tests) it falls back to `/etc/init.d/mwan3 reload` (then `restart`).
- `SetConfig` takes a dedicated **apply lock** covering backup → guard write → generate → staged apply → verify → guard removal, so two concurrent saves cannot interleave (the service's own `mu` guards only the event list).

### 5. Failback behavior

- **30-second hold-down** (`failbackHoldDownDuration`) after failback to a higher-priority interface to reduce flapping (`docs/architecture.md` §6.1).
- The **active uplink is a fallback, not a winner.** `computeActiveInterface` walks the candidates in priority order and returns the first one that has been online past the hold-down; it only falls back to the uplink that is currently active when no candidate has cleared the hold-down. Evaluating "keep whatever is active now" first made the hold-down branch unreachable whenever the active uplink was merely online, so a higher-priority link that recovered never took the connection back — failback could not happen at all.

### 6. Test-double fidelity

`MockUCI.AddSection` must match `uci set config.section=stype`, which **creates or updates** and never fails on an existing section. A stricter mock hides a real divergence: an operator edit inside a `travo_` section made every later save fail with "section already exists" on a mock, and would have silently upserted on a device under the same code shape. **Code that needs duplicate detection must check for existence itself**, inside the config lock — `uci.AddSection`'s error is not that check (see `NetworkService.uciSectionExists`).

## Consequences

- Adding IPv6 must become **ADR amendment** or new ADR with device-tested matrices; do not silently extend rules to `ipv6` without that review.
- Manual edits **inside** a `travo_` section do not retire the section while its candidate is still in the config: it is kept, and the options Travo owns are regenerated from the saved settings (§1.2). The operator is told this via a warning alert. Once the candidate is dropped from the config, the section is no longer generated at all and is left in place as inert mwan3 config that only an operator removes.
- A preserved pre-namespacing or hand-written section is still live mwan3 configuration. It is not reported as an error and not surfaced separately by the failover API; an operator who drops a candidate and sees its old section tracked should read that as "Travo no longer owns this", not as an active member. Surfacing preserved sections in the UI/API is a known follow-up, not part of this change.
- Upgrading from a pre-namespacing build leaves its sections in place. This is the accepted cost of the provenance rule: cleanup is only possible for sections this build wrote, and a pre-namespacing build wrote none of them.

## References

- `backend/internal/services/failover_service.go` (`mayDeleteSection` §1.3, `needsBackupSection` §1.3, `adoptSection` §1.2)
- `backend/internal/uci/mock.go` (test-double fidelity, §6)
- `backend/internal/services/network_service.go` (explicit duplicate checks, §6)
- `backend/internal/models/failover.go`
- `docs/architecture.md` §6.1–6.2
- `docs/plans/connection-failover.md`
