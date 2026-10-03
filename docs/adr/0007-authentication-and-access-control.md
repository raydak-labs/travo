---
title: "ADR 0007: Authentication, JWT, and LAN access control"
status: Accepted
date: 2026-05-14
updated: 2026-10-04
tags: [adr, auth, jwt, rpcd, security, openwrt, sessions]
---

# ADR 0007: Authentication, JWT, and LAN access control

## Status

Accepted.

## Context

On device, the administrative password is the **root** password shared with **LuCI/rpcd**. Travo issues **JWTs** for API sessions, may **blocklist** tokens on logout, and supports optional **IP-based restrictions** for defense in depth on untrusted LANs.

## Decision

### 1. Credential verification

- **Production path**: `AuthService` with **ubus** validates login via **`session.login`** as **`root`** with the supplied password (`NewAuthServiceWithUbus`).
- **Development / mock**: bcrypt hash path (`NewAuthService`) when ubus is not wired.
- On successful rpcd login, Travo can persist a **sealed copy** of the root password for subsequent ubus-backed operations (`rpcd-login.sealed` beside `auth.json`), keyed from material derived from the JWT secret (`rpcd_login_seal.go`).

### 2. JWT sessions — clock-independent by design (2026-07-08)

- Successful login returns a **signed JWT** (HS256, 24h lifetime) carrying a random **`jti`**.
- **Session validity is decided by a monotonic-clock registry** (`session_registry.go`), not by comparing `exp` against the wall clock: travel routers boot with wrong clocks and get large NTP/time-sync jumps, which previously invalidated sessions or locked users out after moving timezones. `time.Since` on the registered issue time is immune to wall-clock changes.
- Tokens with an **unknown `jti`** (issued before a backend restart, or from older builds) fall back to standard `exp` validation with a 2-minute leeway. The registry is deliberately memory-only; the fail direction is safe, and cross-restart **revocation** is the blocklist's job (§4, ADR 0009).
- Login and `GET /auth/session` return a **relative `expires_in` (seconds)**. Clients must count down locally (frontend uses `performance.now()`) and must **never compare server `exp` timestamps against the client clock** — a real expiry surfaces as a 401.
- Protected handlers require **`Authorization: Bearer …`** unless explicitly public. Public endpoints are the ones mounted **outside** the authenticated route group, not the ones a middleware decides to skip: `SetupRoutes` registers `/api/openapi.json`, `/api/v1/auth/login` and `/api/v1/system/time-sync` on the app itself and attaches `AuthService.Middleware()` to the `/api/v1` group that carries everything else. `api.PublicRoutes` is the complete list and `TestAuthCoversEveryRoute` walks the live route table to assert it. Adding to that list is a security change, not a convenience one.
- **Why this is structural (2026-10-04):** the middleware used to skip auth by testing `strings.HasPrefix(c.Path(), "/api/")` against the raw request path. Fiber routes on a lowercased copy of the path but `c.Path()` returns the original, so with Fiber's default case-insensitive routing a request to `/API/v1/system/reboot` matched the registered route while failing the prefix test — the entire `/api/v1` surface, including root SSH key installation and firmware flash, was reachable from the LAN with no token. The middleware now has **no path logic at all**, so it cannot drift from routing, and `fiber.Config.CaseSensitive` is `true` so a case-variant request matches no route rather than reaching a handler.

### 2a. Pre-login time sync

- `POST /api/v1/system/time-sync` stays auth-exempt for bootstrap, but unauthenticated calls are **rate limited** (3/min per IP) and only accepted **while the router clock is implausible** (before the binary build time, `-X main.BuildTime`). Authenticated callers may always sync.
- The "clock was plausible" decision is **sticky and in-process**: `api.TimeSyncGate` latches shut the first time the clock is observed at or after the floor, and `NoteClockSet` re-latches when a value the router has just adopted is itself plausible. The naive `now < buildTime ⇒ allow` check was self-defeating, because whoever can move the clock also controls the gate — rewinding to 1970 re-opened the endpoint indefinitely.
- An **unauthenticated** client time must additionally fall inside a bounded window around the floor: **floor − 30 days … floor + 400 days** (`validateClientTimeWindow`). That rejects both the 1970 rewind (which keeps JWT `exp` fallbacks valid forever) and a far-future value (which breaks TLS, `hwclock -w` and cron, and permanently locks out the pre-login recovery path). Authenticated callers bypass the window: an operator holding a token may set any time.
- **BuildTime invariant:** a released binary **must** be stamped with `main.BuildTime` (RFC3339) via `-ldflags "-X main.BuildTime=…"`, as `scripts/build.sh` does. If `BuildTime` is empty or unparseable, `minPlausibleTime()` falls back to the hard-coded floor `2025-01-01T00:00:00Z` — the endpoint still works, but the plausibility check is materially weaker because it no longer tracks the shipped build, so a missing stamp is a release defect, not a benign default. The release workflow must therefore stamp it (see [ADR 0006](./0006-application-platform-and-api-contract.md)).

### 3. Optional IP allowlist

- CIDR list parsing and middleware enforce **LAN-only** or **administrative subnet** access when configured (`ip_allowlist.go`).
- **Bypass** for minimal endpoints required for bootstrap and health (e.g. **`/api/health`**, **`/api/v1/auth/login`**, **`/api/v1/system/time-sync`**) so a mis-tuned allowlist does not brick login recovery—extend bypass list only with extreme care.

### 4. Rate limiting, blocklist, and session revocation

- Login **rate limiting** and **token blocklist** infrastructure live under `backend/internal/auth/` (`ratelimit.go`, `blocklist.go`); behavior must remain predictable under brute-force attempts.
- Both rate limiters run a **periodic global sweep** (`StartCleanup`) so per-IP maps stay bounded when many distinct source IPs never return.
- **Blocklist persistence shipped.** With a store attached (`NewTokenBlocklistWithStore`) revocations are written through to the `blocklist` bucket and reloaded on startup, so a logout or a password change survives a backend restart instead of resurrecting the token for the rest of its 24 h TTL. Only `sha256(token)` (and `sha256(jti)`, under a `jti:` prefix) ever reaches flash — never a raw token. See [ADR 0009](./0009-persistent-store-bbolt.md). This closed the item previously tracked in [`docs/plans/2026-07-08-auth-hardening.md`](../plans/2026-07-08-auth-hardening.md).
- **Password change revokes every session and rotates the caller's token.** `ChangePassword` ends in `rotateSessions()`: every `jti` in the monotonic registry is revoked (and persisted via `BlockJTI`), and a **replacement token** is returned so the operator stays logged in. The policy floor is the named constant `MinPasswordLength = 8` (a `len(newPassword) < 8` request is rejected).
- **No fallback JWT signing secret.** `randomSecretHex()` returns an error if `crypto/rand` fails instead of degrading to a hard-coded constant, and the caller refuses to start. A predictable HS256 key would let anyone who has read the source forge an admin token, so entropy failure must be fatal, not a default.
- Remaining transport hardening (**WS token transport** — it still travels in the query string) is tracked in [`docs/plans/2026-07-08-auth-hardening.md`](../plans/2026-07-08-auth-hardening.md) and `docs/requirements/tasks_open.md`.

### 5. Storage paths

- **`/etc/trafo/auth.json`** — sealed auth metadata / JWT secret storage (see `config.Config` `AuthConfigPath`).
- **`/etc/travo/travo.db`** — the bbolt key/value store opened beside `auth.json`, holding the persisted token-revocation set and the stats-history ring buffer. Opened with a 5 s timeout; failure degrades to **memory-only** with a warning rather than blocking startup. Retention, flash-write batching and the full bucket table are in [ADR 0009](./0009-persistent-store-bbolt.md).
- **TLS** material may live under `/etc/trafo/tls.crt` / `tls.key` when HTTPS is enabled for the Travo listener.

## Consequences

- Changes to login flow must preserve **rpcd compatibility** and LuCI password rotation semantics (user changes `passwd`, Travo picks up on next login).
- New public endpoints must be reviewed against **IP allowlist** and **OpenAPI** security schemes.
- A second surface grants root access over SSH; it has its own rules in [ADR 0008](./0008-ssh-key-management.md).

## References

- `backend/internal/auth/auth.go` — middleware (default-deny), `rotateSessions`
- `backend/internal/auth/blocklist.go` — hash-only blocklist, store-backed
- `backend/internal/auth/file_store.go` — auth.json, no fallback secret
- `backend/internal/auth/session_registry.go` — monotonic session registry
- `backend/internal/api/time_sync_gate.go` — sticky plausibility latch + client window
- `backend/internal/auth/ip_allowlist.go`
- `backend/internal/auth/rpcd_login_seal.go`
- `backend/internal/config/config.go` — default paths
- [ADR 0009](./0009-persistent-store-bbolt.md) — `travo.db` persistence rules
- [ADR 0008](./0008-ssh-key-management.md) — the other root-credential-granting surface
- `docs/requirements/tasks_done.md` — authentication / IP ACL shipped items
