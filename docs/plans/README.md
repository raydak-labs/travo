---
title: Plans index
description: Searchable catalog of all planning and historical design docs in this directory.
updated: 2026-04-13
tags: [plans, traceability, index]
---

# Plans

All historical and in-progress **design/plan** documents live here (single tree).  
For product truth today use [`docs/architecture.md`](../architecture.md), [`docs/requirements/tasks_open.md`](../requirements/tasks_open.md), and [`docs/requirements/tasks_done.md`](../requirements/tasks_done.md).

YAML **frontmatter** on each plan file carries `title`, `description`, `updated`, and `tags` for search (Obsidian-style).

| File | Summary | Tags |
| ---- | ------- | ---- |
| [`2026-03-26-live-state-refresh-and-wifi-saved-networks.md`](2026-03-26-live-state-refresh-and-wifi-saved-networks.md) | Live UI refresh + saved WiFi profiles | `wifi`, `state`, `ux` |
| [`2026-03-26-vpn-disable-latency-and-dns-forwarding.md`](2026-03-26-vpn-disable-latency-and-dns-forwarding.md) | VPN off path, DNS restore | `vpn`, `dns` |
| [`2026-03-26-wireguard-disable-breaks-internet.md`](2026-03-26-wireguard-disable-breaks-internet.md) | WireGuard disable / default route regression | `wireguard`, `vpn` |
| [`2026-04-11-repeater-radio-policy-and-wifi-ux.md`](2026-04-11-repeater-radio-policy-and-wifi-ux.md) | Repeater radio policy, unified AP UX | `wifi`, `repeater` |
| [`adguard-auto-configure.md`](adguard-auto-configure.md) | AdGuard install + dnsmasq integration | `adguard`, `dns`, `services` |
| [`cicd-pipeline.md`](cicd-pipeline.md) | CI/CD for build/test/package | `cicd`, `deployment` |
| [`connection-failover.md`](connection-failover.md) | Multi-WAN failover | `wan`, `failover`, `network` |
| [`data-usage-tracking.md`](data-usage-tracking.md) | Per-interface data counters | `network`, `metrics` |
| [`deployment-packaging-cicd-plan.md`](deployment-packaging-cicd-plan.md) | Packaging and release pipeline | `deployment` |
| [`fix-all-issues-plan.md`](fix-all-issues-plan.md) | Broad bugfix sweep (historical) | `maintenance` |
| [`fix-review-issues-plan.md`](fix-review-issues-plan.md) | Review follow-ups (historical) | `maintenance` |
| [`hardware-buttons.md`](hardware-buttons.md) | Hotplug / button actions | `hardware`, `ux` |
| [`implementation.md`](implementation.md) | Large cross-cutting implementation guide | `meta`, `guide` |
| [`openwrt-travel-gui-phase-1-complete.md`](openwrt-travel-gui-phase-1-complete.md) | Phase 1 milestone snapshot | `meta` |
| [`openwrt-travel-gui-plan.md`](openwrt-travel-gui-plan.md) | Early product plan | `meta` |
| [`resilience-improvements.md`](resilience-improvements.md) | Recovery / robustness ideas | `reliability` |
| [`tailscale-integration.md`](tailscale-integration.md) | Tailscale on router | `tailscale`, `vpn` |
| [`usb-tethering.md`](usb-tethering.md) | USB WAN tethering | `usb`, `wan` |
| [`ux-overhaul.md`](ux-overhaul.md) | Dashboard / network / system UX | `ux`, `frontend` |
| [`wifi-dual-band-bundling.md`](wifi-dual-band-bundling.md) | Dual-band scan + band switch | `wifi` |
| [`wireguard-adguard-oob-fix-plan.md`](wireguard-adguard-oob-fix-plan.md) | WG + AdGuard out-of-box correctness | `wireguard`, `adguard` |
| [`wireguard-full-networking.md`](wireguard-full-networking.md) | WG zones, routes, split tunnel | `wireguard`, `firewall` |
| [`wireguard_client_openwrt_25.12.md`](wireguard_client_openwrt_25.12.md) | OpenWrt 25.12 WG client notes | `wireguard`, `openwrt` |
