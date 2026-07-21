---
title: UI consistency medium mop
description: Closed 3 MEDIUM verify findings + trivial LOW wifi spacing.
updated: 2026-07-20
branch: fix/ui-consistency
---

# Verify mop report

## Status

**DONE** — all 3 MEDIUM items from `_verify-ui-consistency.md` closed; trivial LOW wifi panel spacing fixed.

## Fixes

1. **SQM loading** — `sqm-section.tsx`: Loading paragraph → 3 Skeleton bars.
2. **Input label** — `input.tsx`: embedded Label uses dense default (no `text-sm` / gray-700 override). Login already uses external Label with prominence — unchanged.
3. **Card header icons** — speedtest `h-5`→`h-4`; SSH Keys / Alert Thresholds icons moved to CardHeader siblings with `h-4 w-4 text-gray-500 dark:text-gray-400`; gray-400-only CardHeader icons normalized on guest/schedule/radio/band-switch/mac-policy/split-tunnel.
4. **LOW** — `wifi-wireless-panel.tsx`: `space-y-6`.

## Tests

`cd frontend && pnpm test` → **48 files, 262 tests passed**.

## Commit / push

Message: `fix(ui): close remaining consistency mediums`
