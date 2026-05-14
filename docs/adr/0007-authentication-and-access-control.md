---
title: "ADR 0007: Authentication, JWT, and LAN access control"
status: Accepted
date: 2026-05-14
tags: [adr, auth, jwt, rpcd, security, openwrt]
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

### 2. JWT sessions

- Successful login returns a **signed JWT** (HS256) with bounded expiry (24h in current implementation).
- Protected handlers require **`Authorization: Bearer …`** unless explicitly public.

### 3. Optional IP allowlist

- CIDR list parsing and middleware enforce **LAN-only** or **administrative subnet** access when configured (`ip_allowlist.go`).
- **Bypass** for minimal endpoints required for bootstrap and health (e.g. **`/api/health`**, **`/api/v1/auth/login`**, **`/api/v1/system/time-sync`**) so a mis-tuned allowlist does not brick login recovery—extend bypass list only with extreme care.

### 4. Rate limiting and blocklist

- Login **rate limiting** and **token blocklist** infrastructure live under `backend/internal/auth/` (`ratelimit.go`, `blocklist.go`); behavior must remain predictable under brute-force attempts.

### 5. Storage paths

- **`/etc/travo/auth.json`** — sealed auth metadata / JWT secret storage (see `config.Config` `AuthConfigPath`).
- **TLS** material may live under `/etc/travo/tls.crt` / `tls.key` when HTTPS is enabled for the Travo listener.

## Consequences

- Changes to login flow must preserve **rpcd compatibility** and LuCI password rotation semantics (user changes `passwd`, Travo picks up on next login).
- New public endpoints must be reviewed against **IP allowlist** and **OpenAPI** security schemes.

## References

- `backend/internal/auth/auth.go`
- `backend/internal/auth/ip_allowlist.go`
- `backend/internal/auth/rpcd_login_seal.go`
- `backend/internal/config/config.go` — default paths
- `docs/requirements/tasks_done.md` — authentication / IP ACL shipped items
