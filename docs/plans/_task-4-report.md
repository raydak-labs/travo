# Task 4 Report — CardTitle override cleanup

**Status:** done
**Commit:** `refactor(ui): unify CardTitle sizing across pages`  
**Branch:** `fix/ui-consistency`  
**Worktree:** `.worktrees/ui-consistency`

## Changes

- Removed redundant `text-sm font-medium` / `text-base font-medium` from CardTitle call sites (default already `text-sm font-medium`)
- **71** call sites cleaned across **61** files
- Kept layout classes: `flex items-center gap-2`, `text-center`, etc.
- Kept Login exception: `text-2xl`
- Kept Setup non-size theme classes: `text-center text-gray-500`
- Speedtest: dropped `text-base font-medium` → bare `<CardTitle>`
- SSH Keys / Alert Thresholds: unchanged (`flex items-center gap-2` only; inherit default size)
- Dashboard SourceCard: kept `flex … text-slate-100`; Quick status: kept `flex …` only
- Did not touch loading/empty/error/labels or `mockServiceWorker.js`

## Remaining CardTitle className (post-cleanup)

| Site | className | Notes |
|------|-----------|--------|
| Login | `text-2xl` | documented exception |
| Setup | `text-center text-gray-500` | non-size theme/layout |
| Dashboard SourceCard | `flex items-center gap-2 text-slate-100` | layout + forced-dark title |
| Dashboard Quick status | `flex items-center gap-2` | layout |
| SSH Keys | `flex items-center gap-2` | layout |
| Alert Thresholds | `flex items-center gap-2` | layout |
| card.test.tsx | `flex items-center gap-2` | layout merge test |

No remaining `text-sm` / `text-base` / `font-medium` sizing drift on CardTitle call sites.

## Test summary

```
cd frontend && pnpm test
→ Test Files  48 passed (48)
→ Tests       260 passed (260)
```

## Concerns

- Setup CardTitle still light-only `text-gray-500` (no `dark:` pair) — pre-existing; out of Task 4 scope (kept as instructed).
- SourceCard title still overrides color via `text-slate-100` (needed for forced-dark chrome from Task 3).
