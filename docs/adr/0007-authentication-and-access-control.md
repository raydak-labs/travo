---
title: "ADR 0007: Authentication, JWT, and LAN access control"
status: Accepted
date: 2026-05-14
updated: 2026-07-08
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
- Tokens with an **unknown `jti`** (issued before a backend restart, or from older builds) fall back to standard `exp` validation with a 2-minute leeway. The registry is deliberately memory-only; the fail direction is safe.
- Login and `GET /auth/session` return a **relative `expires_in` (seconds)**. Clients must count down locally (frontend uses `performance.now()`) and must **never compare server `exp` timestamps against the client clock** — a real expiry surfaces as a 401.
- Protected handlers require **`Authorization: Bearer …`** unless explicitly public.

### 2a. Pre-login time sync

- `POST /api/v1/system/time-sync` stays auth-exempt for bootstrap, but unauthenticated calls are **rate limited** and only accepted **while the router clock is implausible** (before the binary build time, `-X main.BuildTime`). Authenticated callers may always sync. An attacker on the LAN can no longer move a healthy clock.

### 3. Optional IP allowlist

- CIDR list parsing and middleware enforce **LAN-only** or **administrative subnet** access when configured (`ip_allowlist.go`).
- **Bypass** for minimal endpoints required for bootstrap and health (e.g. **`/api/health`**, **`/api/v1/auth/login`**, **`/api/v1/system/time-sync`**) so a mis-tuned allowlist does not brick login recovery—extend bypass list only with extreme care.

### 4. Rate limiting and blocklist

- Login **rate limiting** and **token blocklist** infrastructure live under `backend/internal/auth/` (`ratelimit.go`, `blocklist.go`); behavior must remain predictable under brute-force attempts.
- Both rate limiters run a **periodic global sweep** (`StartCleanup`) so per-IP maps stay bounded when many distinct source IPs never return.
- Remaining transport hardening (blocklist persistence, WS token transport, token storage) is tracked in [`docs/plans/2026-07-08-auth-hardening.md`](../plans/2026-07-08-auth-hardening.md).

### 5. Storage paths

- **`/etc/travo/auth.json`** — sealed auth metadata / JWT secret storage (see `config.Config` `AuthConfigPath`).
- **TLS** material may live under `/etc/travo/tls.crt` / `tls.key` when HTTPS is enabled for the Travo listener.

## Consequences

- Changes to login flow must preserve **rpcd compatibility** and LuCI password rotation semantics (user changes `passwd`, Travo picks up on next login).
- New public endpoints must be reviewed against **IP allowlist** and **OpenAPI** security schemes.

## References

- `backend/internal/auth/auth.go`
- `backend/internal/auth/session_registry.go` — monotonic session registry
- `backend/internal/auth/ip_allowlist.go`
- `backend/internal/auth/rpcd_login_seal.go`
- `backend/internal/config/config.go` — default paths
- `docs/requirements/tasks_done.md` — authentication / IP ACL shipped items
