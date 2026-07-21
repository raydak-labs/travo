---
title: Nested cards and form chrome (Approach A)
description: Equal-height WiFi cards, flatten nested panels, align form controls to shadcn-like rules.
updated: 2026-07-20
tags: [frontend, ux, cards, forms]
---

# Nested cards / form chrome — Approach A

**Goal:** Fix reported WiFi/Network layout weirdness and apply shared inset pattern to same-family siblings; strip true Card-in-Card on Services.

**Rules:** `docs/plans/_shadcn-ui-patterns.md` + audit `docs/plans/_ui-nested-cards-audit.md`.

## Tasks

1. Add shared `CardInset` (or `cn` helper class string) for nested regions — `rounded-md border border-gray-200 p-3 dark:border-white/10` and optional muted variant without competing shadow.
2. Equal-height Current Connection / Saved Networks.
3. Scan/Hidden button fit (wrap + shorter labels).
4. Flatten Auto-reconnect, AP nested boxes, guest/MAC/schedule same pattern.
5. DNS Add `h-10`; Diagnostics tab-bar style + Run `h-10`.
6. Services: remove nested Card shadow (ServiceCard flat or `shadow-none`).
7. Tests + deploy to 192.168.1.1.

Worktree: `.worktrees/ui-consistency` branch `fix/ui-consistency`.
