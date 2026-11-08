---
title: Plans index
description: Normative plans only. Unfinished work is a GitHub issue; shipped plans are deleted.
updated: 2026-11-08
tags: [plans, traceability, index]
---

# Plans

This directory holds **normative plans only**: standing specifications that something
else links to as the source of truth for a rule. Unfinished work is a GitHub issue, not
a plan. A shipped plan is deleted, not archived: before it goes, its normative content
moves into the ADR that owns the subsystem ([ADR 0013](../adr/0013-operational-invariants-and-device-findings.md)).
Git is the history.

For product truth today use [`docs/architecture/overview.md`](../architecture/overview.md),
the [open issues](https://github.com/raydak-labs/travo/issues) and
[`docs/requirements/tasks_done.md`](../requirements/tasks_done.md).

YAML **frontmatter** on each plan file carries `title`, `description`, `updated`,
`status` and `tags` for search (Obsidian-style).

| Plan                                                                                                     | Status    | Summary                                                                                                | Tags                   |
| -------------------------------------------------------------------------------------------------------- | --------- | ------------------------------------------------------------------------------------------------------ | ---------------------- |
| [`2026-07-24-frontend-traveler-ia.md`](2026-07-24-frontend-traveler-ia.md)                               | normative | Information architecture for non-technical users: sidebar-only nav, tap budget, auto-expand precedence | `frontend`, `ux`, `ia` |
| [`2026-07-25-sidebar-defaults-lighter-disclosure.md`](2026-07-25-sidebar-defaults-lighter-disclosure.md) | normative | Per-page disclosure table: which sections are visible by default and which are collapsed               | `frontend`, `ux`, `ia` |

## Rules

- A plan that becomes a standing specification rather than a unit of work keeps
  `status: normative` and stays listed here. Everything else is a GitHub issue.
- Do not edit plan bodies for drive-by cleanup. Editing a plan to match changed
  behaviour destroys the record of why the change was made.
- Every plan added here must appear in the table above: a Go test enforces it
  (`TestPlansIndexListsEveryPlan` in `backend/internal/docsgate/docs_index_test.go`).
