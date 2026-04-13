---
title: Documentation map
description: Entrypoint for human and agent docs; links to architecture, backlog, workflows, plans.
updated: 2026-04-13
tags: [docs, index, navigation]
---

# Documentation Map

## Read first

- [`AGENTS.md`](../AGENTS.md) — repo workflow, guardrails, finish criteria
- [`docs/architecture.md`](./architecture.md) — stable architecture and safety decisions

## Requirements and work status

| What | Where |
| ---- | ----- |
| Open backlog | [`docs/requirements/tasks_open.md`](./requirements/tasks_open.md) |
| Completed log | [`docs/requirements/tasks_done.md`](./requirements/tasks_done.md) |
| Wireless invariants, safety | [`docs/architecture.md`](./architecture.md#2-wireless-model-and-invariants) |
| Legacy checkbox snapshot (read-only) | [`docs/_archive/requirements_done.md`](./_archive/requirements_done.md) |

**Backlog:** [`tasks_open.md`](./requirements/tasks_open.md) · **Open questions:** [`tasks_open.md`](./requirements/tasks_open.md#open-questions)

## Workflow docs

- [`docs/development.md`](./development.md) — local dev workflow
- [`docs/deployment.md`](./deployment.md) — install, packaging, deployment
- [`docs/testing.md`](./testing.md) — on-device checks (scripts + full playbook)
- [`docs/ui-theming.md`](./ui-theming.md) — frontend theming rules and tokens

## Plans and history

- **All plans:** [`docs/plans/`](./plans/) (single tree). Searchable catalog: [`docs/plans/README.md`](./plans/README.md).
- **Do not** edit plan bodies for drive-by cleanup; they are reference/history. Metadata lives in YAML frontmatter and in `docs/plans/README.md`.

## Other

- [`INIT_PROMPT.md`](../INIT_PROMPT.md) — initial project prompt
