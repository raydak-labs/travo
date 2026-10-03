---
title: Open tasks
description: Active product and engineering backlog; link target for plans and architecture.
updated: 2026-09-28
tags: [backlog, requirements, tasks]
---

# Open Tasks

Working backlog only — no duplicate “priority queue”; each item appears once under its area.

Stable rules: [`../architecture.md`](../architecture.md). Shipped work: [`tasks_done.md`](./tasks_done.md).

> **Last updated:** 2026-09-28 (the `updated:` field in the frontmatter is the same date; keep them in step)

## 1. WiFi Management

### 1.3 WiFi Modes

- [ ] Mesh / WDS mode

## 2. Network Management

### 2.2 Connected Clients

- [ ] Client bandwidth limiting (QoS per device)
- [ ] Parental controls / client group policies

### 2.5 USB Tethering

- [ ] Bluetooth tethering

## 3. VPN Management

### 3.3 General VPN UX

- [ ] OpenVPN support

## 4. Services Management

### 4.6 Future Services

- [ ] Cloudflared (Cloudflare Tunnel) *(registered — needs on-device test)*
- [ ] Watchcat (connection watchdog) *(registered — needs on-device test)*

## 6. Dashboard And Monitoring

### 6.2 Real-Time Monitoring

- [ ] Historical data beyond the current window. The 6 h ring buffer (30 s interval,
      720 points, persisted in `/etc/travo/travo.db`) shipped; longer retention is a
      product decision about flash wear — see [ADR 0009](../adr/0009-persistent-store-bbolt.md)
      for the write-batching rule any extension must follow.

## 7. Authentication And Security

- [ ] Two-factor authentication
- [ ] WebSocket token in a header or `Sec-WebSocket-Protocol` instead of the query
      string, where it lands in access logs and browser history (ADR 0007 §4).

## 11. Advanced Networking

- [ ] mDNS / Bonjour forwarding (Chromecast, AirPlay across network segments)
- [ ] Custom routing rules
- [ ] VLAN configuration

## 12. UX And UI Polish

- [ ] Multi-language support (i18n)

## 13. Deployment And Packaging

- [ ] Automatic updates mechanism

## 14. Hardware Buttons

- [ ] Custom button action scripting
- [ ] Long-press vs short-press differentiation. See [Hardware Buttons plan](../plans/hardware-buttons.md#phase-4--long-press-vs-short-press-future).

## 15. Follow-Ups From The 2026-09-26 Critical Review

These are real doc↔code gaps that the review surfaced and that the remediation pass
documented rather than fixed. They are code changes, not doc changes.

- [ ] **SSH key audit trail.** `POST /api/v1/system/ssh-keys` grants root SSH access and
      currently writes nothing to the log ([ADR 0008](../adr/0008-ssh-key-management.md)).
- [ ] **`DeleteSSHKey` is positional.** The index space is positional, so a delete after
      an add shifts later keys. A fingerprint-addressed delete is a breaking API change.
- [ ] **Store degradation is invisible.** If `/etc/travo/travo.db` cannot be opened the
      backend only logs a warning; the UI cannot tell that history and cross-restart
      revocations are not persisting ([ADR 0009](../adr/0009-persistent-store-bbolt.md)).

## 16. Held Dependency Upgrades

Consolidating the Renovate branches on 2026-10-02 surfaced three upgrades that could not land
at the time. Each remaining one is pinned by an `allowedVersions` rule in
`.github/renovate.json5` so Renovate stops proposing it; drop the matching rule once the
blocker clears. The TypeScript 7 entry has since been cleared by moving off ESLint to
oxlint — see [`tasks_done.md`](./tasks_done.md) § "Frontend Toolchain".

- [ ] **MSW 3 — blocked by Vitest 5.** Every `@vitest/mocker` 5.x release through `5.0.3`
      peer-requires `msw ^2.4.9`, so Vitest 5 and MSW 3 are mutually exclusive. We chose
      Vitest 5, so MSW stays on 2.x. Unblocking MSW 3 also needs the `onUnhandledRequest` →
      `onUnhandledFrame` rename in `frontend/src/main.tsx` and `frontend/src/test/setup.ts`.
      Watch: `peerDependencies.msw` on https://registry.npmjs.org/@vitest/mocker.
- [ ] **jsdom 30.1 — blocked by Vitest 5.** Vitest 5's jsdom compat shim resolves jsdom's
      implementation symbol with `getOwnPropertySymbols(new Blob())[0]`, assuming the first
      own symbol is the impl symbol. jsdom 30.1.0 switched wrapper registration to
      `registerWrapper(wrapper, impl, interfaceDescriptor)`, so the lookup now resolves the
      descriptor instead and any request whose body contains a `File` throws
      `Cannot read properties of undefined (reading '_buffer')` before MSW sees it.
      Watch: a Vitest release that resolves the impl symbol explicitly.

## 17. Research And Open Questions

- [x] ~~Investigate whether a lightweight database such as SQLite makes sense for Travo
      passwords and collected data such as CPU or traffic usage.~~ **Decided and
      implemented as bbolt** (`/etc/travo/travo.db`, ADR 0009). Measured: bbolt adds
      **247 KB** to the stripped binary (12.57 → 12.83 MB) where `modernc.org/sqlite`
      would add ~10 MB. The current consumers are the token-revocation set and the
      stats-history ring buffer.

### Open Questions

1. Multi-radio strategy: what is the best default radio assignment on travel routers with 2+ radios?
2. WAN vs WWAN: how should they coexist, and is metric-based routing enough?
3. Startup safety net: should install always ensure an AP is broadcasting so users cannot lock themselves out?
4. AdGuard DNS setup: should install move dnsmasq to `5353`, or should AdGuard forward to dnsmasq?
5. Repeater mode implications: should same-radio repeater performance loss be surfaced more aggressively to users?
6. GL.iNet feature parity targets: which multi-WAN, VPN policy, and remote-management ideas are actually in scope?
