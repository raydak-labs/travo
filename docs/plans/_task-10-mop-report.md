# Task 10 mop — residual UI consistency

**Status:** done  
**Branch:** `fix/ui-consistency`  
**Worktree:** `.worktrees/ui-consistency`  
**Commit message:** `fix(ui): mop residual consistency gaps from audit`

## Fixed (MEDIUM)

| ID | Fix |
|----|-----|
| M1 | SourceCard empties unified to slate `<p className="text-slate-500">` (Ethernet matched Repeater/USB; dropped light EmptyState on forced-dark chrome) |
| M2 | Replaced undefined shadcn tokens (`text-muted-foreground`, `border-border`, `bg-muted/*`, `text-foreground`, `border-input`, `bg-background`, `ring-ring`, `text-destructive`) with theme-safe gray/border/input pairs |
| M3 | `DataUsageSection` loading → Card + Skeleton (no more `return null`) |
| M4 | Swept `text-gray-500` → add `dark:text-gray-400` across `frontend/src` secondary/hint/icon chrome |
| M5 | ipv6 / led-stealth Label-like headers got dark pairs in sweep |
| M6 | Data Usage “not installed” → `EmptyState` |

## Fixed (LOW, quick)

| ID | Fix |
|----|-----|
| L1 | Setup `CardTitle` → `text-gray-500 dark:text-gray-400` |

Left alone: L2 radio wrappers, L3 captive status rows, L4/L5 semantic red.

## Docs

- Indexed `2026-07-20-ui-consistency.md` in `docs/plans/README.md`

## Tests

```text
cd frontend && pnpm test
Test Files  48 passed (48)
Tests       262 passed (262)
```

## Out of scope / untouched

- `frontend/public/mockServiceWorker.js`
- Status color semantics (emerald/red)
- SourceCard forced-dark slate body text (kept `text-slate-*`)
