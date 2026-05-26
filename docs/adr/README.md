---
title: Architecture Decision Records
description: Durable decisions that complement docs/architecture.md with topic-specific detail.
updated: 2026-05-26
tags: [adr, architecture]
---

# Architecture Decision Records

ADRs capture **durable, topic-specific** decisions that are too long for `docs/architecture.md` but are still normative for implementers.

| ID | Title | Status |
| -- | ----- | ------ |
| [0001](./0001-dns-vpn-captive-portal-architecture.md) | DNS paths, VPN, captive portal, and restore semantics | Accepted |
| [0002](./0002-wireless-model-and-luci-apply.md) | Wireless model, health, and LuCI-style UCI apply | Accepted |
| [0003](./0003-crash-guards-and-live-state.md) | Crash guards for automated live-state changes | Accepted |
| [0004](./0004-firewall-zones-and-interface-policy.md) | Firewall zones, forwarding, and interface topology | Accepted |
| [0005](./0005-multi-wan-failover-mwan3.md) | Multi-WAN connection failover (mwan3) | Accepted |
| [0006](./0006-application-platform-and-api-contract.md) | Application platform, repo shape, and API contract | Accepted |
| [0007](./0007-authentication-and-access-control.md) | Authentication, JWT, and LAN access control | Accepted |
| [0008](./0008-wireguard-family-protocol-coexistence.md) | WireGuard-family protocol coexistence and AmneziaWG support | Accepted |

**How to use:** pick the ADR that matches the subsystem you are changing; if the behavior is not documented yet, add or amend an ADR in the same numbering series.

When an ADR supersedes or narrows a summary in `docs/architecture.md`, both stay in sync: the overview file links here; the ADR is the source of truth for that topic.

**Obsidian:** open the vault at `docs/` and use **`+ Start here.md`** for a wikilink map of the same files; CLI search/read examples are in `AGENTS.md` → *Documentation retrieval (Obsidian CLI)*.
