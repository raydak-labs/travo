---
title: Open tasks
description: Active product and engineering backlog; link target for plans and architecture.
updated: 2026-10-04
tags: [backlog, requirements, tasks]
---

# Open Tasks

Working backlog only — no duplicate “priority queue”; each item appears once under its area.

Stable rules: [`../architecture/overview.md`](../architecture/overview.md). Shipped work: [`tasks_done.md`](./tasks_done.md).

> **Last updated:** 2026-10-04 (the `updated:` field in the frontmatter is the same date; keep them in step)

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
- [ ] **Summary band on the remaining top-level pages.** WiFi has one
      ([ADR 0012](../adr/0012-ui-consistency-and-status-language.md)); Network, System, Services and
      VPN do not. The band is the template: always visible, flat tiles, per-tile degradation. Roll it
      out one page at a time, reusing each page's existing queries.
- [ ] **Live radio state is still missing.** The WiFi Radios tile shows the *configured* radio
      (UCI-derived), not what the radio is doing on the air. A `/api/v1/wifi/radios/status`
      endpoint (country, actual channel/width, TX power, noise, bitrate) would fill it without
      changing the tile contract. Needs device validation on the ath11k/IPQ6018 target.

## 13. Deployment And Packaging

- [ ] Automatic updates mechanism

## 14. Hardware Buttons

- [ ] Custom button action scripting. Any persisted button→action map must live under
      `/etc/trafo/` with the rest of the ordinary state; the earlier plan text named
      `/etc/openwrt-travel-gui/buttons.json`, which predates the path unification and
      must not be recreated ([ADR 0013](../adr/0013-operational-invariants-and-device-findings.md) rule 6).
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

## 16. Follow-Ups From The 2026-10-04 Deep Review Remediation

Raised by the device verification pass and the five remediation lanes. Each is a deliberate
decision, not an oversight; the reasoning is in the linked ADR or lane report.

- [ ] **Captive restore-on-reconnect is no longer on the read path.** `GET /api/v1/captive/status`
      no longer mutates state, so a bypass that outlives the captive state (internet back, portal
      gone) now waits for the operator to press restore or for the 5-minute startup restore. If
      that gap matters, the right home is a bounded scheduler with its own crash guard — not a
      mutation inside a GET ([ADR 0001](../adr/0001-dns-vpn-captive-portal-architecture.md) §4).
- [ ] **Preserved mwan3 leftovers are not surfaced.** Under the recorded-ownership rule a legacy
      section for a dropped candidate survives. `GET /api/v1/failover` does not report it, so the
      operator sees trailing config with no explanation
      ([ADR 0005](../adr/0005-multi-wan-failover-mwan3.md) §1).
- [ ] **Recovery from an already-corrupt dnsmasq layer record.** Writes are now atomic, but a
      record corrupt before that change still fails loudly rather than being quarantined.
- [ ] **Route table gap: `POST /api/v1/vpn/wireguard/profiles/:id/activate`.** `handlers.ts`
      hardcodes it; `shared/src/api/routes.ts` does not describe it. Listed explicitly in the
      parity test's `ROUTE_TABLE_GAPS` until the route table gains it.
- [ ] **No gate pins action SHAs or image digests.** `renovate.json5` sets `pinDigests`, which
      re-pins a dropped digest but does not fail a build when one is missing. All current `uses:`
      are SHA-pinned; nothing enforces it.

- [ ] **`Connect` can disable the caller's OWN access point.** Connecting the uplink to the
      band the operator is standing on disables the AP they are on, while another band survives.
      That is a disruption, not a strand, so the lockout rule correctly stays silent — but the
      operator gets no acknowledgement. `guardLockoutExcluding(excludeRadio)` is exactly the
      mechanism; `Connect` does not take a `LockoutRequest`. Needs a product decision first.
- [ ] **Banner freshness is evaluated once at render.** A stale adopt-warning that is on screen
      when the page goes quiet keeps its wording until the next reload, and the clock-skew slack
      is one-directional (a future-dated timestamp can read as fresh indefinitely).
- [ ] **Unpinned edges.** The overwrite-warning freshness window boundary has no test, and the
      `Connect`/`Disconnect`/`Reconcile` "keeps an access point up" assertions do not pin the
      caller classification they depend on at the HTTP boundary.

## 17. Held Dependency Upgrades

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

## 17b. On-Device Verification Of The 2026-10-04 UI Remediation

- [ ] **Confirm the USB-tether uplink on real hardware.** The backend now surfaces `network.usbtether`
      in `status.Interfaces` and treats it as the effective WAN when it is the only uplink. All
      frontend coverage is unit-level; the failure mode it fixes ("dashboard says No Internet while
      phone-tethered") was reported from source reading, not reproduced. Tether a phone with no
      ethernet and confirm `WAN Status` shows the USB row active.
- [ ] **Confirm the dialog animations actually appear.** `animate-in` / `zoom-in-95` /
      `slide-in-from-*` were dead class strings: `index.css` never imported `tw-animate-css`, so the
      mobile drawer and every modal snapped instead of animating. The plugin is now imported. Dead
      utilities do not fail lint or tsc — only a device or a look at `dist/assets/*.css` shows this.
- [ ] **Check dark mode on native controls.** `color-scheme` is now declared, which changes how
      `type="time"`, `type="range"`, `type="file"` inputs and scrollbars render. Verify the LED and
      Wi-Fi schedule pickers and the firmware file input in dark mode.

## 18. Research And Open Questions

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
7. **`RepeaterWizard` is implemented but mounted nowhere** (`frontend/src/components/wifi/repeater-wizard/`;
   no non-test file imports it). Repeater setup today is the generic path — mode switch, then
   `useWifiConnect`, then the AP save — as three independent apply windows with no cross-operation
   rollback, which is the exact failure `use-repeater-wizard.ts` `rollback()` was written to prevent
   (ADR 0002 §2: an AP and the STA on the same PHY can crash ath11k/IPQ6018). Two of its settings
   (`allow_ap_on_sta_radio`) are reachable *only* inside it. **Decide: mount it behind the repeater
   mode switch, or delete it.** The UI copy no longer points users at it in the meantime — the
   2026-10-04 UI review found copy telling operators to enable a setting in a screen that does not
   exist. Not decided here because deleting working rollback logic and adding a new entry point are
   both beyond a UI-review remediation.
