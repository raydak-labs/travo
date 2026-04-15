---
title: Open tasks
description: Active product and engineering backlog; link target for plans and architecture.
updated: 2026-04-13
tags: [backlog, requirements, tasks]
---

# Open Tasks

Working backlog only — no duplicate “priority queue”; each item appears once under its area.

Stable rules: [`../architecture.md`](../architecture.md). Shipped work: [`tasks_done.md`](./tasks_done.md).

> **Last updated:** 2026-04-13

## 1. WiFi Management

### 1.3 WiFi Modes

- [ ] Mesh / WDS mode

### 1.4 Multi-Radio Support

- [ ] Startup script to auto-discover radio setup and persist config

## 2. Network Management

### 2.2 Connected Clients

- [ ] Client bandwidth limiting (QoS per device)
- [ ] Parental controls / client group policies

### 2.5 USB Tethering

- [ ] Bluetooth tethering

### 2.7 Connection Failover

- [x] Priority-based WAN source (Ethernet > WiFi > USB Tether). See [Connection Failover plan](../plans/connection-failover.md).
- [x] Health check via periodic ping (configurable target). See [Connection Failover plan](../plans/connection-failover.md).
- [x] Auto-switch to next source on failure. See [Connection Failover plan](../plans/connection-failover.md).
- [x] Notification on failover event. See [Connection Failover plan](../plans/connection-failover.md).
- [x] Dashboard integration showing active failover interface. See [Connection Failover plan](../plans/connection-failover.md).

## 3. VPN Management

### 3.3 General VPN UX

- [ ] OpenVPN support

## 4. Services Management

### 4.6 Future Services

- [ ] Cloudflared (Cloudflare Tunnel)
- [ ] Watchcat (connection watchdog)

## 6. Dashboard And Monitoring

### 6.2 Real-Time Monitoring

- [ ] Historical data (store and display last hours/days)

## 7. Authentication And Security

- [ ] Two-factor authentication

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

## 16. Research And Open Questions

- [ ] Investigate whether a lightweight database such as SQLite makes sense for Travo passwords and collected data such as CPU or traffic usage. Evaluate footprint, flash usage, durability, and operational benefit before committing.

### Open Questions

1. Multi-radio strategy: what is the best default radio assignment on travel routers with 2+ radios?
2. WAN vs WWAN: how should they coexist, and is metric-based routing enough?
3. Startup safety net: should install always ensure an AP is broadcasting so users cannot lock themselves out?
4. AdGuard DNS setup: should install move dnsmasq to `5353`, or should AdGuard forward to dnsmasq?
5. Repeater mode implications: should same-radio repeater performance loss be surfaced more aggressively to users?
6. GL.iNet feature parity targets: which multi-WAN, VPN policy, and remote-management ideas are actually in scope?
