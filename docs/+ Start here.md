---
title: Travo documentation hub
description: Obsidian-first map of the docs vault; same files as Git-rendered markdown.
aliases:
  - Travo docs hub
  - documentation hub
tags:
  - docs
  - hub
  - travo
updated: 2026-10-05
---

# Travo documentation hub

> **Git / IDE:** this vault lives at `docs/` in the repo. The canonical markdown map is still [[README]].

## Normative (read before changing behavior)

- [[architecture/overview|architecture]] — invariants, safety, pointers into ADRs
- [[adr/README]] — ADR index (source of truth by topic)
- [[adr/0001-dns-vpn-captive-portal-architecture]]
- [[adr/0002-wireless-model-and-luci-apply]]
- [[adr/0003-crash-guards-and-live-state]]
- [[adr/0004-firewall-zones-and-interface-policy]]
- [[adr/0005-multi-wan-failover-mwan3]]
- [[adr/0006-application-platform-and-api-contract]]
- [[adr/0007-authentication-and-access-control]]
- [[adr/0008-ssh-key-management]]
- [[adr/0009-persistent-store-bbolt]]
- [[adr/0010-uci-write-serialisation-and-request-contracts]]
- [[adr/0011-frontend-lint-toolchain-oxlint-and-typescript-7]]
- [[adr/0012-ui-consistency-and-status-language]]
- [[adr/0013-operational-invariants-and-device-findings]]

## Backlog and shipped work

- [[requirements/tasks_open]]
- [[requirements/tasks_done]]

## Guides (how to do something)

- [[guides/development]] — local dev workflow
- [[guides/deployment]] — install, packaging, deployment
- [[guides/testing]] — how to run the checks

## Reference (what things are)

- [[reference/ui-theming|ui-theming]] — frontend tokens and component contracts

## Plans (live and normative work)

- [[plans/README]] — catalog of `plans/`; shipped plans are deleted once folded into ADRs

## On-device verification

- [[tests/failover-verification]]
- [[tests/on-device-verification]] — standing playbook; start here before any wireless, DNS/VPN,
  guard or install change touches a device

## Archive

- [[_archive/requirements_done]] — legacy exhaustive export (read-only)
