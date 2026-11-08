---
title: Documentation map
description: Entrypoint for human and agent docs; the layout rules that keep this vault navigable.
updated: 2026-11-08
tags: [docs, index, navigation]
---

# Documentation Map

This directory is an **Obsidian vault** (`.obsidian/`). For graph and wikilink
navigation, open **`+ Start here.md`**. For the rules that govern what goes where,
see [`AGENTS.md`](../AGENTS.md) § Documentation.

## Read first

- [`AGENTS.md`](../AGENTS.md) — repo workflow, guardrails, documentation rules
- [`architecture/overview.md`](./architecture/overview.md) — stable architecture and safety decisions
- [`adr/README.md`](./adr/README.md) — architecture decision records, indexed by topic
- [GitHub issues](https://github.com/raydak-labs/travo/issues) — the current backlog

## Layout

| Directory                                                | Holds                                                                              | Rule                                                                                                                |
| -------------------------------------------------------- | ---------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| [`adr/`](./adr/README.md)                                | Normative decisions, one numbered file per topic                                   | A decision belongs here once it is hard to reverse, surprising without context, and the result of a real trade-off. |
| [`architecture/overview.md`](./architecture/overview.md) | Cross-cutting invariants and safety rules that predate or span ADRs                | Point to ADRs instead of restating them.                                                                            |
| [`guides/`](./guides/)                                   | How to _do_ something: develop, deploy, test                                       | Task-shaped, not topic-shaped.                                                                                      |
| [`reference/`](./reference/)                             | What something _is_: tokens, component contracts, tables                           | Read before changing the thing described.                                                                           |
| [`tests/`](./tests/)                                     | On-device verification playbooks                                                   | Every playbook must be linked from somewhere in `docs/`; a Go test enforces it.                                     |
| [`agents/`](./agents/)                                   | Config the engineering skills read: issue tracker, triage labels, domain-doc rules | Edit directly; no decisions or plans here.                                                                          |
| [`images/`](./images/)                                   | Screenshots and assets referenced from docs and the repo README                    | Binary assets only; no prose.                                                                                       |

## Current pointers

| Question                                 | Where                                                                                                                |
| ---------------------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| Is my change safe on a device?           | [`architecture/overview.md`](./architecture/overview.md), then the ADR for the subsystem                             |
| What is decided about X?                 | [`adr/README.md`](./adr/README.md)                                                                                   |
| What is the state of the work?           | [GitHub issues](https://github.com/raydak-labs/travo/issues)                                                         |
| How do I build/deploy?                   | [`guides/development.md`](./guides/development.md), [`guides/deployment.md`](./guides/deployment.md)                 |
| How do I verify on hardware?             | [`guides/testing.md`](./guides/testing.md), then the playbook in [`tests/`](./tests/)                                |
| Which backend package owns X?            | [`reference/backend-packages.md`](./reference/backend-packages.md)                                                   |
| Which UI primitive or token do I use?    | [`reference/ui-theming.md`](./reference/ui-theming.md), [ADR 0012](./adr/0012-ui-consistency-and-status-language.md) |
| What did we decide about a device quirk? | [ADR 0013](./adr/0013-operational-invariants-and-device-findings.md)                                                 |
| What are we doing next?                  | [GitHub issues](https://github.com/raydak-labs/travo/issues)                                                         |

## Terminology

- [`GLOSSARY.md`](../GLOSSARY.md) — the words this project uses with a specific
  meaning (summary band, stale, configured vs live radio, traffic history).

Shipped history, planning documents and the historical review reports were removed.
Open work is GitHub issues; the removed text is in git history (commit `3cee6c6` and earlier).
