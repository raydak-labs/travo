# Task 5 Report — Loading → Skeleton

**Status:** done  
**Commit hash:** `ce527f86f688e912f39d7d4438e6b5fd860d07f1`  
**Commit:** `refactor(ui): standardize loading with Skeleton`  
**Branch:** `fix/ui-consistency`  
**Worktree:** `.worktrees/ui-consistency`

## Changes

- Replaced `animate-pulse` placeholder `<div>`s with `<Skeleton className="h-16 w-full" />` in:
  - `frontend/src/pages/wifi/wifi-schedule-card.tsx`
  - `frontend/src/pages/wifi/mac-policy-card.tsx`
  - `frontend/src/pages/vpn/split-tunnel-card.tsx`
- Replaced ssh-keys `"Loading…"` text with `<Skeleton className="h-16 w-full" />` in `frontend/src/pages/system/ssh-keys-card.tsx` (loading only; empty-state left for Task 6)
- Left existing `Skeleton` call sites untouched
- Left `Loader2` / button busy states untouched
- Left `system-quick-links-card` button label `Loading…` (fetch busy on AdGuard link, not card content placeholder)
- Did not touch EmptyState / InlineError / `mockServiceWorker.js`

## Post-grep

| Pattern | Remaining |
|---------|-----------|
| `animate-pulse` outside Skeleton | none (only `skeleton.tsx` + speedtest test asserting Skeleton’s class) |
| Card content `"Loading…"` | none |
| Button busy `"Loading…"` | `system-quick-links-card` AdGuard fetch label (intentional) |

## Test summary

```
cd frontend && pnpm test
→ Test Files  48 passed (48)
→ Tests       260 passed (260)
```

No test updates required (no assertions on old pulse markup / `"Loading…"` for these cards).

## Concerns

- `system-quick-links-card` still shows text `Loading…` on button while AdGuard config fetches — out of Task 5 card-content scope; could later use `Loader2` for consistency with other busy buttons.
- Skeleton height `h-16` matches prior pulse blocks; denser multi-line skeletons elsewhere already use stacked `h-4`/`h-10` patterns — fine to leave as-is.
