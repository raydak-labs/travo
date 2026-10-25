---
title: Backend packages
description: What each Go package under backend/internal owns and which way the imports go.
updated: 2026-10-25
tags: [docs, backend, go, packages]
---

# Backend packages

`backend/cmd/server` parses flags, sets up logging and runs the listen/shutdown loop. Everything
else lives under `backend/internal/`.

| Package | Owns |
| ------- | ---- |
| `app` | Wiring: builds every service, the Fiber app, the routes and the background workers (`app.New`, `app.Lifecycle`) |
| `api` | HTTP handlers, request validation, the OpenAPI contract |
| `auth` | Login, JWT sessions, token blocklist, rate limits, IP allowlist |
| `ws` | The WebSocket hub that pushes live status |
| `network` | Interfaces, WAN, DHCP, DNS, clients and their classification, port forwards, DDNS, SQM, USB tethering, data usage, traffic history, uptime |
| `wifi` | Radios, access points, uplink STA, repeater, guest WiFi, MAC, schedules, band switching, the lockout guard |
| `uplink` | VPN (WireGuard, Tailscale), AdGuard Home DNS and the captive-portal bypass, plus the dnsmasq resolver layer stack they share ([ADR 0001](../adr/0001-dns-vpn-captive-portal-architecture.md)) |
| `failover` | Multi-WAN failover through mwan3 ([ADR 0005](../adr/0005-multi-wan-failover-mwan3.md)) |
| `system` | System info and stats, logs, backup and restore, firmware, LEDs, buttons, SSH keys, alerts, stats history, speed test, package installs |
| `platform` | Shared OpenWrt plumbing: the command runner, UCI config locks, rpcd apply/confirm, atomic file writes, the connectivity prober |
| `guard` | Crash guards ([ADR 0003](../adr/0003-crash-guards-and-live-state.md)) |
| `uci`, `ubus` | The UCI and ubus clients, with in-memory mocks for mock mode and tests |
| `execx` | `os/exec` with mandatory timeout tiers |
| `store` | The bbolt store ([ADR 0009](../adr/0009-persistent-store-bbolt.md)) |
| `models` | JSON request and response types |
| `config`, `docsgate` | Runtime configuration; the tests that keep `docs/` honest |

## Import direction

Domain packages import `platform`, `guard`, `uci`, `ubus`, `execx` and `models`, never `api` or
`app`. Between domains the imports are `wifi → network`, `system → wifi` and
`failover → network, system`; anything else would be a cycle. `api` imports the domains, and
`app` imports everything.

Each service has one constructor. Larger ones take a `Deps` struct whose zero values are the
production defaults (`wifi.New`, `network.New`, `failover.New`, `uplink.NewVpnService`,
`uplink.NewCaptiveService`); small ones take explicit arguments where `nil` means the real
implementation. Test doubles that other packages need live in a package's `mock.go` or in a
`…test` package such as `network/networktest`.
