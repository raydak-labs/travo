---
title: "Sidebar defaults + lighter disclosure"
description: "Parent group navigates to daily default; arrow toggles submenu; fewer PageSections"
updated: 2026-07-25
tags: [plan, frontend, ux, ia]
---

# Sidebar defaults + lighter disclosure

> **Status:** Approved (device feedback follow-up to traveler IA)  
> **Supersedes (partial):** traveler IA leaf labels Extras/Tools → **Advanced**; page disclosure “collapse by default on Advanced pages” → only rare/power stay collapsed.

## Sidebar

| Group | Label click → | Leaves | Arrow |
|-------|---------------|--------|-------|
| WiFi | `/wifi` Connect | Connect · Advanced | toggle only |
| Network | `/network` Status | Status · Internet & LAN · Advanced | toggle only |
| Services | `/services` Apps | Apps · Tailscale · Speedtest · SQM? | toggle only |
| System | `/system` Settings | Settings · Logs | toggle only |

- Group row body = `Link` to `defaultTo`; does **not** toggle.
- Chevron = expand/collapse only.
- **`defaultTo` is omitted from the submenu** (parent label *is* that page). WiFi submenu = Advanced only; Network = Internet & LAN · Advanced; Services = Tailscale · …; System = Logs.
- Child route (incl. default) → auto-expand group.
- Storage key bump: `otg-sidebar-groups-v3`.

## Labels

- `/wifi/advanced`: **Advanced** (was Extras)
- `/network/advanced`: **Advanced** (was Tools)
- Twin “Advanced” OK — parent group disambiguates.

## Page disclosure (strategy A)

`PageSection` only for rare / power / geek. Everyday cards = normal visible `Card`s. Optional `h2` cluster headings (not accordions).

| Page | Always visible | Collapsed `PageSection` |
|------|----------------|-------------------------|
| WiFi Advanced | Guest, MAC clone, Band switching, Schedule, DNS tools | Radio hardware, Repeater radio layout, MAC policy |
| Network Internet & LAN | WAN, Interfaces, LAN, DHCP&DNS, Local DNS, Reservations, Leases | — |
| Network Advanced | Failover, USB tethering, Data usage, Diagnostics, Speed test | DDNS, Firewall, IPv6, DoH, WoL |
| VPN | WireGuard, Split tunnel, AdGuard hint | Verify, DNS leak, VPN speed |
| System | Glance + configuration cards | SSH keys, Backup, Firmware, Danger zone |

## Out of scope

- Moving cards between routes
- Changing collapsed icon rail destinations
