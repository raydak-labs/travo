---
title: Auth hardening follow-ups
description: Small plan for the remaining auth-transport weaknesses — blocklist persistence, WS token transport, token storage — after the clock-independent session work.
updated: 2026-07-08
tags: [auth, security, sessions, websocket]
---

# Auth hardening follow-ups

Status: **proposed** (small plan; none of these block release).

## Context

The 2026-07-08 hardening branch already landed the structural fixes:

- Sessions are tracked in a **monotonic-clock registry** keyed by JWT `jti`
  (`backend/internal/auth/session_registry.go`) — validity is immune to wall
  clock jumps, NTP syncs, and timezone fixes. Unknown `jti` (issued before a
  backend restart) falls back to `exp` with a 2-minute leeway.
- Login and `GET /auth/session` return **relative** `expires_in` seconds; the
  frontend counts down via `performance.now()` and never logs out from local
  clock math (`frontend/src/hooks/use-session-timeout.ts`).
- Unauthenticated `POST /system/time-sync` is rate limited and only accepted
  while the router clock is implausible (before the binary build time).
- Login and time-sync rate limiter maps are swept periodically.

Three known weaknesses remain. Each is listed with impact, proposal, effort.

## 1. Token blocklist is memory-only

**Impact:** Logout revocation is lost on backend restart while tokens live
24h. The session registry is also memory-only, but its *fail direction* is
safe (unknown jti → exp fallback), whereas a blocklisted token becomes valid
again after restart.

**Proposal:** Persist blocked token hashes (SHA-256, not raw tokens) with
their expiry to `/etc/travo/revoked.json` next to `auth.json`; load on start,
prune on the existing cleanup tick. Few writes (only logout), tiny file.

**Effort:** ~half a day incl. tests. **Priority: medium.**

## 2. WebSocket token travels as a query parameter

**Impact:** `/api/v1/ws?token=…` can land in intermediate proxy or debug
logs. On a LAN-only admin device with TLS optional this is low severity, but
it is the only place a bearer token appears in a URL.

**Proposal:** Issue a short-lived (30s, single-use) WS ticket from an
authenticated `POST /auth/ws-ticket`, pass the ticket in the query string
instead of the real token. Alternative (less code): accept the token via
`Sec-WebSocket-Protocol` header, which browsers can set.

**Effort:** ~1 day incl. frontend. **Priority: low.**

## 3. JWT stored in localStorage

**Impact:** Any XSS gives token theft. Standard SPA tradeoff; the app has no
third-party scripts and a strict same-origin API, which bounds exposure.

**Proposal:** Move to an `HttpOnly; SameSite=Strict; Secure` cookie session
and CSRF token for mutating requests. This is a larger change (affects CORS
config, WS auth, `Authorization` header clients like the OpenAPI consumers)
— only worth doing together with a deliberate API-versioning pass.

**Effort:** 2–3 days. **Priority: low — document, don't rush.**

## Non-goals

- Multi-user accounts / roles (single-admin device).
- Refresh tokens: 24h sessions with re-login are acceptable for a router UI.

## Order

1 → 2 → 3. Item 1 is the only one with a realistic failure story
(restart-resurrected token on a shared/hostile LAN).
