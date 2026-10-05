---
title: Plans index
description: Live and normative plans. Shipped plans are deleted once their decisions are folded into an ADR.
updated: 2026-10-05
tags: [plans, traceability, index]
---

# Plans

This directory holds **live and normative plans only**. A plan that has shipped
is deleted, not archived: before it goes, its normative content moves into the
ADR that owns the subsystem ([ADR 0013](../adr/0013-operational-invariants-and-device-findings.md)
records the cross-cutting findings). Executed plans are history; the code, the
ADR and `docs/requirements/tasks_done.md` are truth.

For product truth today use [`docs/architecture/overview.md`](../architecture/overview.md),
[`docs/requirements/tasks_open.md`](../requirements/tasks_open.md) and
[`docs/requirements/tasks_done.md`](../requirements/tasks_done.md).

YAML **frontmatter** on each plan file carries `title`, `description`, `updated`,
`status` and `tags` for search (Obsidian-style).

| Plan | Status | Summary | Tags |
| ---- | ------ | ------- | ---- |
| [`2026-07-08-auth-hardening.md`](2026-07-08-auth-hardening.md) | active | Auth follow-ups: WebSocket token transport, HttpOnly cookie + CSRF migration (blocklist persistence shipped, see [ADR 0007](../adr/0007-authentication-and-access-control.md)) | `auth`, `security`, `sessions`, `websocket` |
| [`2026-07-24-frontend-traveler-ia.md`](2026-07-24-frontend-traveler-ia.md) | normative | Information architecture for non-technical users: sidebar-only nav, tap budget, auto-expand precedence | `frontend`, `ux`, `ia` |
| [`2026-07-25-sidebar-defaults-lighter-disclosure.md`](2026-07-25-sidebar-defaults-lighter-disclosure.md) | normative | Per-page disclosure table: which sections are visible by default and which are collapsed | `frontend`, `ux`, `ia` |
| [`2026-09-28-critical-review-remediation.md`](2026-09-28-critical-review-remediation.md) | superseded | 2026-09-26 critical code review remediation. Open items carried in [`tasks_open.md` §15](../requirements/tasks_open.md) and §16; completion marks in this plan are unreliable | `review`, `remediation`, `traceability` |
| [`hardware-buttons.md`](hardware-buttons.md) | active | Hardware button detection and custom actions. Detection reads devicetree, not `/etc/rc.button` ([ADR 0013](../adr/0013-operational-invariants-and-device-findings.md) rule 6) | `hardware`, `ux` |
| [`wifi-dual-band-bundling.md`](wifi-dual-band-bundling.md) | active | Dual-band scan grouping and per-connection band choice | `wifi`, `bands`, `repeater` |

## Rules

- A plan names work that has **not** shipped. If the work shipped, close it out:
  fold the normative parts into the owning ADR, then delete the file.
- A plan that becomes a standing specification rather than a unit of work (the two
  IA plans) keeps a `status: normative` and says so here; it is not deleted,
  because something links to it as the source of truth for a rule.
- Do not edit plan bodies for drive-by cleanup. Editing a plan to match changed
  behaviour destroys the record of why the change was made.
- Every plan added here must appear in the table above: a Go test enforces it
  (`TestPlansIndexListsEveryPlan` in `backend/internal/services/docs_index_test.go`).
