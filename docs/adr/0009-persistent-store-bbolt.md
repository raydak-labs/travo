---
title: "ADR 0009: Persistent key/value store (bbolt at /etc/travo/travo.db)"
status: Accepted
date: 2026-09-28
tags: [adr, storage, bbolt, flash, persistence, auth, stats]
---

# ADR 0009: Persistent key/value store (bbolt at `/etc/travo/travo.db`)

## Status

Accepted.

## Context

Two pieces of state must survive a backend restart but do not belong in UCI or in
UCI-adjacent JSON:

- **Token revocations.** A logged-out or password-rotated session must stay dead
  across `/etc/init.d/travo restart` (ADR 0007). A memory-only set resurrects every
  revoked token for its remaining TTL.
- **Stats history.** The dashboard's CPU/memory/throughput ring buffer should not
  start empty on every boot.

Both are small, append-or-replace, and read far more often than written. A **full
relational engine** is disproportionate on a device whose whole budget is a
~12.8 MB stripped binary and NAND-backed flash.

The SQLite-vs-alternative question was an explicit backlog item
(`docs/requirements/tasks_open.md` §16). It was **measured, not guessed**: on this
repo, `go.etcd.io/bbolt` adds **247 KB** to the stripped binary (12.57 → 12.83 MB),
while `modernc.org/sqlite` would add roughly **10 MB**; flat JSON files cannot do a
transactional multi-key write and need hand-rolled locking. bbolt won on footprint
and on transactional `Update`. See
[`docs/plans/2026-07-08-hardening-followups-and-persistence.md`](../plans/2026-07-08-hardening-followups-and-persistence.md)
(Q4) for the measurement.

## Decision

### 1. One store, one file, opened once

- `backend/internal/store` is a thin bbolt wrapper exposing string-keyed buckets:
  `Put` / `Get` / `Delete` / `ForEach`, each a single bbolt transaction. A missing
  bucket is **not** an error (`Get` returns `nil`, `ForEach` iterates nothing) — a
  fresh device and a bucket created by a later version must behave identically.
- The file is **`<dir of AuthConfigPath>/travo.db`**
  (`backend/cmd/server/main.go`). Production default:
  **`/etc/travo/travo.db`**, opened with mode `0600` and a **5 s open timeout** so a
  stale file lock from a hard kill cannot block startup forever. Deriving the path
  from `AuthConfigPath` is deliberate: a test with a temp auth path gets a temp
  store, with no second knob to keep in sync.
- The handle is owned by `appLifecycle` and closed **last**, after the HTTP server
  has drained and after `statsHistory.Stop()` has flushed into it. Shutdown order is
  `app.ShutdownWithTimeout` → `lifecycle.Stop()` → `db.Close()`; in-flight handlers
  must never touch a closed handle.

### 2. Flash-write discipline (NAND / overlayfs)

`/etc/travo` is an overlayfs on NAND. Every write is a write amplification event, and
unnecessary ones wear the flash and cost seconds of stall. Therefore:

- **Batched writes are the default.** Stats history keeps its ring buffer in memory
  and flushes to the store **every 20 collects** (`flushEvery`), i.e. roughly every
  **10 minutes** at the 30 s sample interval — not once per sample — plus a final
  flush in `Stop()`.
- **Rare writes may write through.** A logout or password change is a rare,
  user-initiated event; paying one write for durability is correct, and the
  alternative (a logout that a redeploy resurrects) is a security bug.
- **Nothing writes on a read path.** `Get` / `ForEach` are `View` transactions and
  never dirty a page.
- **The rule for new consumers:** if a new store-backed feature is sampled, polled or
  per-request, it must batch. If it is event-driven and rare, it may write through.
  A new consumer that writes per sample/per request is a defect, not a style choice.

### 3. What is persisted, and what deliberately is not

| Bucket | Key | Value | Retention | Write policy |
| ------ | --- | ----- | --------- | ----------- |
| `blocklist` | `sha256(token)` hex, and `jti:` + `sha256(jti)` | token expiry, unix seconds | until the token's own `exp`; pruned every 5 min by `StartCleanup`, and pruned at open | write-through (rare, security-relevant) |
| `stats_history` | `points` | JSON array of `{time, cpu, memory, rx_bytes, tx_bytes}` | newest **720** points (30 s interval → **~6 hours**); older points are trimmed on restore | batched: every 20 collects, plus `Stop()` |

- **Only hashes are stored, never credentials.** A `jti` is not a credential on its
  own, but it is hashed too, so the on-disk contents are uniformly hashes and a flash
  dump cannot be replayed as a token.
- **The session registry stays memory-only, on purpose.** `SessionRegistry` decides
  token lifetime from a monotonic clock and is a cache of *live* sessions; persisting
  it would buy nothing that the blocklist does not already provide, and a restored
  registry would resurrect sessions across a reboot. Durability for revocations is
  the blocklist's job (ADR 0007).
- **No other data goes in the store.** Device configuration, credentials and DNS/VPN
  snapshots stay in UCI and in the `/etc/trafo/*.json` files described in
  [ADR 0001](./0001-dns-vpn-captive-portal-architecture.md) and
  [ADR 0003](./0003-crash-guards-and-live-state.md). The store is for **derived,
  regenerable, security-relevant state** only.

### 4. Degradation on open failure

If `store.Open` fails, the backend **logs a warning and runs memory-only**; it does
not `log.Fatal` (`main.go`). Concretely:

- `statsHistory` is constructed via the non-store constructor, so the ring buffer
  simply starts empty and never persists.
- `blocklist` is constructed via the non-store constructor, so revocations apply to
  the current process lifetime and a restart would resurrect them.
- Everything else — login, session issuance, wireless, network, services — is
  completely unaffected. Losing history or cross-restart revocation is a degraded
  experience; refusing to start would brick the web UI on a device the user may be
  able to reach only through that UI.

The failure direction is therefore chosen knowingly: **availability over
durability**, with the security-relevant cost (revocation not surviving a restart)
accepted only for a device whose UI is the recovery path.

## Consequences

- A second store consumer (data-usage budgets, alert history) needs a **new bucket**,
  not a new file, and needs an explicit retention row in §3 of this ADR.
- Because degradation is silent to the UI, a store that never opens looks exactly
  like one that works with an empty history. The startup warning is the only signal.
- The DB is not a backup target: `RestoreBackup` / `FactoryReset` do not treat it as
  authoritative. Losing it costs a few hours of history, not configuration.

## Open questions

- Startup currently only **warns**. Surfacing "history is not persisting" in the UI
  (or in `GET /api/health`) would make the degradation visible; not decided here.
- The measured bbolt overhead (247 KB) has not been re-measured since; the ~12 MB
  binary figure in the 2026-07-08 plan is the reference point.

## References

- `backend/internal/store/store.go` — the wrapper and its flash-write contract
- `backend/cmd/server/main.go` — `store.Open`, memory-only fallback, lifecycle
  ownership and shutdown order
- `backend/internal/auth/blocklist.go` — `blocklist` bucket, hashing, retention
- `backend/internal/services/stats_history.go` — `stats_history` bucket, `flushEvery`
- `docs/architecture.md` §8 — device constraints
- [ADR 0007](./0007-authentication-and-access-control.md) — sessions, revocations
- [`docs/plans/2026-07-08-hardening-followups-and-persistence.md`](../plans/2026-07-08-hardening-followups-and-persistence.md)
  — the footprint measurement behind the bbolt decision
