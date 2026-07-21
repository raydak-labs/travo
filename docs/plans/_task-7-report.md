# Task 7 Report — Error UX → InlineError

**Status:** DONE  
**Commit:** `1ac60d63cd164bd0dc4e103e40cfba9bd9d68809`  
**Branch:** `fix/ui-consistency`  
**Worktree:** `/Users/marbaced/projects/raydak/travo/.worktrees/ui-consistency`

## Changes

- Migrated load/display error chrome to `InlineError` (children API; no `message` prop):
  - `speedtest-page.tsx` — status load error + run mutation error; wrap error/`!status` in `space-y-6`
  - `speedtest-page.tsx` — `!status` after load → `EmptyState` (no more `return null`)
  - `diagnostics-card.tsx` — diagnostics run error
  - `speed-test-card.tsx` — network speed test error
  - `vpn-speed-test-card.tsx` — VPN speed test error
  - `login-form-fields.tsx` — root auth error (kept AlertCircle via children + `flex` className)
  - `sqm-section.tsx` — failed SQM config load
- Updated `speedtest-page.test.tsx` (role=alert, empty state, run error)

## Skipped (intentional)

- Field-level validation `<p role="alert">` / `text-red-*` (not load/display boxes)
- Sonner toasts
- Status banners / confirm dialogs (`wifi-health-banner`, `confirm-radio-disable-dialog`, etc.)
- Loading / EmptyState / Label migrations (other tasks)
- `frontend/public/mockServiceWorker.js`

## Tests

```text
pnpm exec vitest run \
  src/pages/services/__tests__/speedtest-page.test.tsx \
  src/pages/login/__tests__/login-page.test.tsx \
  src/pages/services/__tests__/sqm-page.test.tsx \
  src/components/ui/__tests__/inline-error.test.tsx \
  src/pages/vpn/__tests__/vpn-page.test.tsx
→ 5 files, 26 tests passed
```

## Concerns

- Diagnostics / network / VPN speed-test error text now uses InlineError `text-sm` (was `text-xs` in a few cards) — slight size bump, intentional per primitive contract.
- Login root error dark bg was `dark:bg-red-950/50` / `dark:text-red-400`; now shared InlineError `dark:bg-red-950` / `dark:text-red-300`. Login tests still assert `border-red` / `bg-red` only.
- Full `make lint` / `make test` / `make build` deferred to Task 10 per plan cadence.
