---
title: UI consistency pass
description: Unify page shell, cards, loading/empty/error, labels, and status chrome across Travо frontend.
updated: 2026-07-20
tags: [frontend, ux, theming, consistency]
---

# UI Consistency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Travо frontend UI/UX consistent across every page — shared card chrome, page rhythm, loading/empty/error patterns, form labels, and status badges — without changing product behavior.

**Architecture:** Tighten design-system defaults in `frontend/src/components/ui/` first, then migrate call sites. Protected pages keep `AppShell`; page roots follow a documented spacing contract. Prefer existing primitives (`Skeleton`, `EmptyState`, `Badge`) over one-offs.

**Tech Stack:** React + TypeScript + Vite + TailwindCSS v4, existing `components/ui/*`.

## Global Constraints

- Work only in worktree `/Users/marbaced/projects/raydak/travo/.worktrees/ui-consistency` on branch `fix/ui-consistency`.
- Do not change backend, OpenAPI, or device scripts.
- Follow `docs/ui-theming.md`: inherit body text; secondary copy uses `text-gray-500 dark:text-gray-400` (or both light/dark pairs); no light-only gray text on dual-theme surfaces.
- Surgical diffs: no drive-by refactors unrelated to consistency.
- TDD where behavior changes are testable; update existing frontend tests that assert class names/copy.
- Commit after each task with Conventional Commits (`fix(ui):` / `feat(ui):` / `refactor(ui):`).
- Run `cd frontend && pnpm test` (or targeted) after each task; before finish run top-level `make lint`, `make test`, `make build` from worktree root.
- Do **not** remove Network Status “Connected Clients” table (product duplication OK); only align presentation with shared primitives.
- Login/Setup stay outside AppShell (intentional).
- Model for all subagents: `cursor-grok-4.5-high`.

### Page content contract (normative)

| Page type | Root class |
|-----------|------------|
| Default protected page | `space-y-6` |
| Tabbed page (WiFi, Network) | `space-y-4` (tab bar tight) then panels `space-y-6` |
| Never | Extra page-level `p-*` (AppShell already pads) |
| Lazy suspense fallback | No extra `p-*` (shell already pads) |

### Card contract (normative)

- `CardTitle` default: `text-sm font-medium leading-none tracking-tight` + existing theme text colors (not `text-lg font-semibold`).
- Remove redundant `className="text-sm font-medium"` overrides after default change (keep layout classes like `flex items-center gap-2`).
- Exceptions allowed: Login hero title (`text-2xl`); Setup step titles that are not CardTitle.
- Card structure: prefer `Card` → `CardHeader` → `CardTitle` → `CardContent`; do not invent alternate title typography on Card surfaces.

### Feedback contract (normative)

- **Loading:** `Skeleton` (or Spinner/`Loader2` only for in-button/submit busy). Kill `animate-pulse` placeholder blocks and plain `"Loading…"` text for card content.
- **Empty:** `EmptyState` from `@/components/ui/empty-state`.
- **Load/query errors in cards/pages:** shared `InlineError` (new) — red bordered box with message.
- **Mutation success/failure:** keep `sonner` toasts where already used; do not convert toasts to InlineError or vice versa for the same interaction type.
- **Status:** prefer shared `Badge` variants; dashboard SourceCard connection chip may stay custom but must be theme-safe (light + dark).

### Label contract (normative)

- New `Label` primitive: default `text-xs font-medium text-gray-500 dark:text-gray-400`.
- Dense forms migrate to `<Label>` (or same classes). Login may keep `text-sm font-medium` via `Label` `className` override if needed for prominence.

---

## Task 1: Design-system primitives

**Files:**
- Modify: `frontend/src/components/ui/card.tsx` (`CardTitle` default)
- Create: `frontend/src/components/ui/label.tsx`
- Create: `frontend/src/components/ui/inline-error.tsx`
- Create/modify tests under `frontend/src/components/ui/__tests__/` if present, else add minimal tests
- Update: `docs/ui-theming.md` with Card/Label/feedback contracts (short)

- [ ] Write/adjust unit tests for CardTitle default classes
- [ ] Change `CardTitle` default to `text-sm font-medium …` (keep `text-gray-900 dark:text-white`)
- [ ] Add `Label` forwardRef wrapping `<label>` with contract classes + `cn`
- [ ] Add `InlineError` (`role="alert"`, `text-sm`, red border/bg pairs for light+dark)
- [ ] Document contracts in `docs/ui-theming.md`
- [ ] Commit: `feat(ui): tighten card title, label, inline-error primitives`

---

## Task 2: Page shell contract

**Files:**
- `frontend/src/pages/clients/clients-page.tsx` — drop extra `p-4`
- `frontend/src/components/layout/lazy-page-boundary.tsx` — drop fallback `p-4 sm:p-6`
- `frontend/src/pages/services/sqm-page.tsx` — wrap content in `space-y-6` when needed
- Tabbed pages: confirm WiFi/Network roots stay `space-y-4`; other pages `space-y-6`
- Update any tests asserting clients padding

- [ ] Fix clients double padding
- [ ] Fix lazy boundary double padding
- [ ] Normalize SQM page root spacing
- [ ] Grep for page roots with `p-4`/`p-6` under `pages/` and remove extras inside AppShell
- [ ] Commit: `fix(ui): normalize page shell spacing`

---

## Task 3: Dashboard SourceCard theme safety

**Files:**
- `frontend/src/pages/dashboard/dashboard-page.tsx` (`SourceCard` + related slate-on-white)

- [ ] Make SourceCard readable in light mode: either force dark surface (`bg-slate-900` both themes) OR map slate text to gray/slate with proper light/dark pairs
- [ ] Prefer forcing dark card chrome for SourceCard (matches topology aesthetic) — set explicit `bg-slate-900 border-slate-700 text-slate-*` without relying on white Card default
- [ ] Update dashboard tests if they assert classes
- [ ] Commit: `fix(ui): fix dashboard source card light contrast`

---

## Task 4: CardTitle override cleanup (consistent cards)

**Files:** all CardTitle call sites under `frontend/src/`

- [ ] After Task 1 default change, remove redundant `text-sm font-medium` / `text-base font-medium` overrides that only duplicated sizing
- [ ] Fix outliers that relied on old `text-lg` default without override (SSH Keys, Alert Thresholds) — they should now match `text-sm` automatically; keep icon+flex classes
- [ ] Speedtest: drop `text-base` override so it matches board
- [ ] Login may keep `text-2xl` on CardTitle
- [ ] Grep remaining `CardTitle className=` and ensure no sizing drift except documented exceptions
- [ ] Commit: `refactor(ui): unify CardTitle sizing across pages`

---

## Task 5: Loading → Skeleton

**Files:** cards using `animate-pulse` placeholders or `"Loading…"` text (e.g. split-tunnel, mac-policy, wifi-schedule, ssh-keys-card)

- [ ] Replace pulse placeholders with `Skeleton`
- [ ] Replace ssh-keys `"Loading…"` with `Skeleton`
- [ ] Leave `Loader2` on buttons/dialogs
- [ ] Commit: `refactor(ui): standardize loading with Skeleton`

---

## Task 6: EmptyState adoption

**Files:** clients empty table, logs empty, ssh keys empty, setup scan empty, any other one-off empties

- [ ] Replace one-off empty copy with `<EmptyState message="…" />`
- [ ] Keep icon optional
- [ ] Update tests that look for old empty markup
- [ ] Commit: `refactor(ui): use EmptyState for empty content`

---

## Task 7: Error UX → InlineError

**Files:** speedtest, diagnostics, network speed-test, login root error, SQM failed load, etc.

- [ ] Replace ad-hoc red error boxes / bare error paragraphs for **load/display** errors with `InlineError`
- [ ] Do not change toast-based mutation feedback
- [ ] Avoid `return null` on speedtest when status missing after load — show InlineError or EmptyState as appropriate
- [ ] Commit: `refactor(ui): standardize inline errors`

---

## Task 8: Form Label migration

**Files:** dense forms across network/wifi/vpn/system using `text-xs text-gray-500` labels

- [ ] Migrate form field labels to `Label` component (or identical classes)
- [ ] Ensure `dark:text-gray-400` present
- [ ] Login: use Label with `className="text-sm font-medium text-gray-700 dark:text-gray-300"` if needed
- [ ] Commit: `refactor(ui): migrate form labels to Label primitive`

---

## Task 9: Status chrome + shell titles

**Files:**
- Dashboard SourceCard status chip — theme-safe (Task 3 may cover)
- `logs-level-badge.tsx` — align with `Badge` if straightforward
- `frontend/src/router.tsx` (or title map) — consistent shell titles for sub-routes (`WiFi` stays flat OK; Services subpages keep `Services / X` OR document; prefer breadcrumb style for service children only)
- Align custom success/fail dots to use consistent emerald/red pairing with dark variants where missing

- [ ] Prefer Badge for log levels if visual parity OK; else leave LogsLevelBadge but ensure dark pairs
- [ ] Normalize any remaining status pills that break light mode
- [ ] Commit: `refactor(ui): align status chrome and shell titles`

---

## Task 10: Verify + docs index

- [ ] Update `docs/plans/README.md` index row for this plan
- [ ] Run `make lint`, `make test`, `make build` from worktree
- [ ] Fix failures
- [ ] Final commit if docs/fixes needed: `chore(ui): finish consistency verify`

---

## Done when

- No protected page adds its own outer `p-4`/`p-6`
- Card titles visually match across Settings-like cards
- SourceCards readable in light + dark
- No card content `Loading…` / pulse placeholders remain (except button spinners)
- Empty/load-error paths use EmptyState / InlineError
- Labels use shared Label (or identical classes) with dark variants
- Lint, tests, build green
