---
title: "Frontend traveler IA"
description: "Sidebar-only navigation and progressive disclosure for non-technical travel-router users"
updated: 2026-07-24
tags: [plan, frontend, ux, ia]
---

# Frontend traveler IA

> **Status:** Design approved; follow-up: [`2026-07-25-sidebar-defaults-lighter-disclosure.md`](2026-07-25-sidebar-defaults-lighter-disclosure.md) (parent→default, Advanced labels, lighter PageSections)  
> **Implementation plan:** [`2026-07-24-frontend-traveler-ia-implementation.md`](2026-07-24-frontend-traveler-ia-implementation.md)  
> **Goal:** Make daily travel jobs obvious; keep every existing feature reachable without duplicate nav chrome.  
> **Primary user:** Non-technical traveler (hotel WiFi, “am I online?”, VPN on/off).

**Supersedes** the *nav chrome* parts of [`ux-overhaul.md`](ux-overhaul.md) (in-page WiFi/Network tab bars). Normative nav rules also live in [`../ui-theming.md`](../ui-theming.md) § Navigation patterns. Does **not** move cards between routes; disclosure collapses content **where it already lives**.

---

## Decisions (locked)

1. **Persona A** (= primary user above: non-tech traveler) — default chrome for that persona. Power features stay reachable behind collapsed sidebar groups and collapsed page sections.
2. **Sidebar-only app nav** — remove WiFi and Network **in-page tab bars**. Sidebar + header actions are the only app-level navigation.
3. **Always-visible top-level:** Dashboard · WiFi · Network · Clients · VPN · Services · System. Services is its own group (not under System).
4. **First visit (no usable sidebar-group storage):** land on `/dashboard`; all sidebar groups **collapsed**.
5. **Collapse means two things:** (a) sidebar submenus via **chevron only**; (b) page section boxes for **rare/power** cards only — see follow-up plan.
6. **Group label click** navigates to the daily default (WiFi→Connect, Network→Status, Services→Apps, System→Settings). Does not toggle the submenu.
7. **No feature deletion** — every current capability stays on its current route (or existing page section).
8. **Header** = action chrome only (title, status, notifications, theme, reboot/shutdown/logout). No header page menus. **Page title** for WiFi/Network children = sidebar **leaf** label (Connect, Advanced, Status, Internet & LAN, …), not the group name alone. Services children may keep `Services / Tailscale`-style titles if already used.
9. **Cross-links** stay light (Dashboard shortcuts, Status → Clients, VPN ↔ Tailscale hints).

---

## Nav tree

| Top-level | Kind | Default | Sidebar leaves |
|-----------|------|---------|----------------|
| Dashboard | leaf | `/dashboard` | Dashboard |
| WiFi | group | `/wifi` | **Connect** · **Advanced** |
| Network | group | `/network` | Status · **Internet & LAN** · **Advanced** |
| Clients | leaf | `/clients` | Clients |
| VPN | leaf | `/vpn` | VPN |
| Services | group | `/services` | **Apps** · Tailscale · Speedtest · SQM |
| System | group | `/system` | Settings · Logs |

Twin **Advanced** leaves OK (parent disambiguates). Storage key: `otg-sidebar-groups-v3`.

---

## Wayfinding rules

### Sidebar owns route hierarchy

- Switching Status ↔ Internet & LAN ↔ Advanced (Network) or Connect ↔ Advanced (WiFi) is **sidebar only**. Do not reintroduce a page tab strip for those axes.
- **In-page tabs allowed only** when the axis is *not* already a sidebar hierarchy. Today: Logs **System Log / Kernel Log** (log *source*, not a nav destination). That exception must stay documented in `ui-theming.md` so WiFi/Network tabs do not return.

### Active group auto-expand (precedence)

1. Cold load / first visit with **no** usable `localStorage` group state → Dashboard + all groups collapsed.
2. Any navigation to a **group child** (sidebar click, deep link, Dashboard CTA) → **auto-expand** that parent and highlight the leaf. This wins over “start collapsed” on the same load (e.g. open `/wifi/advanced` → WiFi group open).
3. After the user manually toggles a group, persist that choice.

### Sidebar group storage migration

- Today’s key `otg-sidebar-groups` defaults were all **open**; many browsers already store that.
- Implementation **must bump** the sidebar-groups storage key when defaults/behavior change (now `otg-sidebar-groups-v3`). Do not rely on merging new `defaultOpen` into an old all-`true` blob.
- Corrupt JSON → treat as missing → new defaults.

### Tap budget (traveler jobs)

From Dashboard, ≤ **3 taps** (opening the mobile nav sheet counts as one):

1. Start hotel / upstream WiFi connect → `/wifi`.
2. Toggle VPN → `/vpn` or Dashboard quick action.
3. “Am I online?” → Dashboard itself after land (0 further taps).

Regression if an implementation exceeds that.

### Clients canonical

- **Clients** (`/clients`) = full management UI.
- **Network Status** may keep a short preview (**max 5 rows**) + **View all → Clients**.

---

## Page disclosure

Collapse sections **in place** on the route that already owns the card. Do not move cards between Status / Internet & LAN / Tools in this pass.

| Page | Default open | Default collapsed |
|------|--------------|-------------------|
| Dashboard | Topology / WAN sources, quick status, quick actions, throughput | Keep lean |
| WiFi Connect (`/wifi`) | Mode, connection / scan / saved, captive / internet health | Long help only if it crowds the happy path |
| WiFi Extras | Short intro optional | DNS tools, radio HW, guest, MAC, band switch, schedule |
| Network Status | WAN status, clients preview, traffic, uptime | — |
| Network Internet & LAN | **WAN + LAN open**; DHCP, DNS, entries, reservations, leases **collapsed** | (locked — not implementer choice) |
| Network Tools | Failover summary open if present | Remaining advanced cards on that route (firewall, IPv6, DoH, WoL, diagnostics, speed, USB, DDNS, data usage, …) |
| VPN | WireGuard connect + core toggles | Split tunnel, leak / speed / verify |
| Clients | List + kick/block | Heavy reservation editor if needed |
| System | Glance + password / timezone essentials | Maintenance, firmware, SSH, danger zone |
| Services Apps | Install/list UI | Install danger if noisy |
| Logs | Log stream + existing source tabs | Filters as today |

Page-section open/closed **not** persisted in the first implementation (defaults every visit).

**Pre-existing dual homes** (e.g. Network Tools speed card vs Services Speedtest page) stay as-is this pass; do not invent a new canonical rule here.

---

## Cross-links

- Dashboard CTAs navigate to the correct route **and** expand the matching sidebar group.
- Network Status clients → Clients.
- VPN ↔ Tailscale / AdGuard hints stay; Tailscale remains under Services.

---

## Explicitly deferred

- Moving Tailscale under the VPN leaf.
- Fully merging Clients into Network Status.
- Moving cards between Configuration/Internet & LAN and Tools routes.
- Persisted accordion state for page sections.
- Breadcrumbs / command palette.
- LuCI escape-hatch link in System.
- Updating [`2026-07-20-ui-consistency.md`](2026-07-20-ui-consistency.md) “Tabbed page” spacing contract (do when removing tab bars).

---

## Implementation touchpoints (hints)

Not an implementation plan — pointers for the next plan:

- `frontend/src/components/layout/nav-config.ts` — labels, `defaultOpen` all false, storage key bump to `otg-sidebar-groups-v2`.
- `frontend/src/components/layout/sidebar.tsx` / `sidebar-nav-group.tsx` — auto-expand on active child route; keep SQM injected into Services items when installed.
- Stop using `wifi-page-tab-bar.tsx` / `network-page-tab-bar.tsx` (delete or leave unused only if tests require a short transition); keep URL routes.
- Page section collapse on dense / Advanced pages (existing Collapsible/`<details>` patterns OK).
- Clients preview (≤5 rows) + link on Network Status.
- Page titles / drawer copy align with Connect / Internet & LAN / Apps / Extras / Tools (leaf titles for WiFi/Network).
- Tests: first-visit defaults, storage key v2, deep-link expands group, Dashboard CTA expands group, no WiFi/Network tablists mirroring sidebar.

---

## Success criteria

1. WiFi and Network pages have **no** tablist that mirrors sidebar Connect/Internet & LAN/Extras/Tools (or old Wireless/Configuration/Advanced labels).
2. First cold load with empty/migrated storage: Dashboard + all sidebar groups collapsed.
3. Opening `/wifi/advanced` (or any group child) expands that group and highlights the leaf.
4. Labels Connect / Internet & LAN / Apps / Extras / Tools appear in sidebar (and matching titles).
5. All pre-change features still reachable on the same routes/sections.
6. Tap-budget jobs still hold on a mobile-width check.

---

## Related

- [`ux-overhaul.md`](ux-overhaul.md) — historical; nav chrome superseded.
- [`../ui-theming.md`](../ui-theming.md) — normative Navigation patterns (aligned with this plan).
- Peer pattern: GL.iNet admin — task top-level, geek stuff under Network/System, no twin tab chrome for the same axis.
