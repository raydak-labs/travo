---
title: Completed tasks
description: Shipped milestones and done work; pair with tasks_open for current backlog.
updated: 2026-04-15
tags: [backlog, requirements, changelog]
---

# Completed Tasks

High-level **what shipped**, grouped by subsystem. For the old exhaustive checkbox export, see [`../_archive/requirements_done.md`](../_archive/requirements_done.md) (read-only).

When you finish something in [`tasks_open.md`](./tasks_open.md): remove it there, add a short bullet under the right heading here, and update [`../architecture.md`](../architecture.md) if you introduced a new invariant.

> **Last updated:** 2026-04-15

## Milestone checklist (compact)

Closed “Task N” items from earlier tracking — detail lives in the sections below.

- [x] Form pattern standardisation; network page **Status / Configuration / Advanced** grouping
- [x] Captive portal: auto-accept portal terms
- [x] Authentication: IP-based access control
- [x] VPN speed test; DDNS custom update URL; SQM / QoS (traffic shaping)
- [x] WiFi: setup wizard unified AP credentials; repeater-options `PUT` reconcile
- [x] Multi-WAN failover: priority-based uplink switching, health checks, 30s hold-down, alerts

## WiFi And Network Foundation

- Upstream WiFi scan, connect, disconnect, saved-network management, hidden networks, priority ordering.
- WiFi modes: AP, STA, repeater (wizard + health).
- Multi-radio detection; dual-band scan bundling and automatic band switching.
- AP: shared credentials, per-radio enable, guest WiFi, QR, MAC clone/policy, scheduling.
- Clients: aliases, block/kick, DHCP reservations, static IPs.
- Network: WAN status and config, DHCP, LAN DNS, DDNS, firewall summary, port forwarding, IPv6, WoL, traffic charts.
- Data usage tracking; USB tethering.
- Multi-WAN failover: priority-based Ethernet/WiFi/USB uplink switching, health tracking, 30-second hold-down for stable failback, alerts, dashboard integration.

## VPN And Services

- WireGuard: import, toggle, status, peers, kill switch, split tunnel, verification, DNS leak checks, speed test.
- Tailscale: install, auth, peers, exit node, SSH toggle.
- Services: install/remove, start/stop, autostart, progress logs.
- AdGuard Home: install, auto-configure, DNS path, dashboard link, config editor, VPN interplay.
- Dynamic DNS including custom update URLs.

## System, Dashboard, And UX

- Dashboard: live stats, charts, quick actions, alerts, notification history, captive banners.
- System: reboot, shutdown, firmware upgrade, factory reset, hostname, backup/restore, LED, timezone, NTP, password, hardware buttons.
- Logs: system/kernel filters, search, export.
- UI: responsive layout, sidebar + mobile drawer, dark mode, skeletons, onboarding, grouped IA.

## Reliability And Operational Fixes

- Wireless apply: LuCI-style rollback; confirm after reachability; no self-confirm while rollback pending.
- Apply failures surfaced; saved state not reported healthy when runtime is broken.
- Saved upstream WiFi persistence; UI refresh after WiFi/VPN actions.
- WireGuard disable restores connectivity; AdGuard install/DNS path fixes.
- OpenAPI at `GET /api/openapi.json`.
- Packaging, install tarball, uci-defaults, CI workflows.
