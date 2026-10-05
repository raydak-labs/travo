---
title: Architecture Decision Records
description: Durable decisions that complement docs/architecture.md with topic-specific detail.
updated: 2026-09-30
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
| [0008](./0008-ssh-key-management.md) | SSH key management (root `authorized_keys`) | Accepted |
| [0009](./0009-persistent-store-bbolt.md) | Persistent key/value store (bbolt at `/etc/travo/travo.db`) | Accepted |
| [0010](./0010-uci-write-serialisation-and-request-contracts.md) | UCI write serialisation, ordered config locks, and strict request-body contracts | Accepted |
| [0011](./0011-frontend-lint-toolchain-oxlint-and-typescript-7.md) | Frontend lint toolchain (oxlint and TypeScript 7) | Accepted |
| [0012](./0012-ui-consistency-and-status-language.md) | UI consistency — card plane, status language, summary bands, traffic history | Accepted |

**How to use:** pick the ADR that matches the subsystem you are changing; if the behavior is not documented yet, add or amend an ADR in the same numbering series.

When an ADR supersedes or narrows a summary in `docs/architecture.md`, both stay in sync: the overview file links here; the ADR is the source of truth for that topic.

**Obsidian:** open the vault at `docs/` and use **`+ Start here.md`** for a wikilink map of the same files; CLI search/read examples are in `AGENTS.md` → *Documentation retrieval (Obsidian CLI)*.
