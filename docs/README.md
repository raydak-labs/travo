---
title: Documentation map
description: Entrypoint for human and agent docs; the layout rules that keep this vault navigable.
updated: 2026-10-05
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
- [`requirements/tasks_open.md`](./requirements/tasks_open.md) — the current backlog

## Layout

| Directory | Holds | Rule |
| --------- | ----- | ---- |
| [`adr/`](./adr/README.md) | Normative decisions, one numbered file per topic | A decision belongs here once it is hard to reverse, surprising without context, and the result of a real trade-off. Everything else does not. |
| [`architecture/overview.md`](./architecture/overview.md) | Cross-cutting invariants and safety rules that predate or span ADRs | Point to ADRs instead of restating them. |
| [`guides/`](./guides/) | How to *do* something: develop, deploy, test | Task-shaped, not topic-shaped. |
| [`reference/`](./reference/) | What something *is*: tokens, component contracts, tables | Read before changing the thing described. |
| [`tests/`](./tests/) | On-device verification playbooks | Every playbook must be linked from somewhere in `docs/`; a Go test enforces it. |
| [`plans/`](./plans/README.md) | Live and normative plans only | A shipped plan is deleted once its decisions are folded into an ADR. |
| [`requirements/`](./requirements/) | Backlog (`tasks_open`) and shipped log (`tasks_done`) | Remove from one, add to the other, in the same commit. |
| [`images/`](./images/) | Screenshots and assets referenced from docs and the repo README | Binary assets only; no prose. |
| [`_archive/`](./_archive/) | Frozen historical exports: the legacy checkbox dump and the point-in-time review reports | Never a source of truth, never edited, never linked as current. |

## Current pointers

| Question | Where |
| -------- | ----- |
| Is my change safe on a device? | [`architecture/overview.md`](./architecture/overview.md), then the ADR for the subsystem |
| What is decided about X? | [`adr/README.md`](./adr/README.md) |
| What is the state of the work? | [`requirements/tasks_open.md`](./requirements/tasks_open.md) |
| What shipped? | [`requirements/tasks_done.md`](./requirements/tasks_done.md) |
| How do I build/deploy? | [`guides/development.md`](./guides/development.md), [`guides/deployment.md`](./guides/deployment.md) |
| How do I verify on hardware? | [`guides/testing.md`](./guides/testing.md), then the playbook in [`tests/`](./tests/) |
| Which UI primitive or token do I use? | [`reference/ui-theming.md`](./reference/ui-theming.md), [ADR 0012](./adr/0012-ui-consistency-and-status-language.md) |
| What did we decide about a device quirk? | [ADR 0013](./adr/0013-operational-invariants-and-device-findings.md) |
| What are we doing next? | [`plans/README.md`](./plans/README.md) |

## Terminology

- [`GLOSSARY.md`](../GLOSSARY.md) — the words this project uses with a specific
  meaning (summary band, stale, configured vs live radio, traffic history).

## Other

- [`_archive/init-prompt.md`](./_archive/init-prompt.md) — the initial project prompt
- [`_archive/reviews/`](./_archive/reviews/) — the 2026-09-26 and 2026-10-04 code reviews.
  Frozen point-in-time reports: a Go test still resolves the paths they cite and requires
  them to exist, but they describe paths and states as they were on that date. Do not
  treat their references as current.
