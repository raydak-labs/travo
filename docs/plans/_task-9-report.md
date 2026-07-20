# Task 9 Report — Status chrome + shell titles

**Status:** done  
**Commit:** `refactor(ui): align status chrome and shell titles`  

**Branch:** `fix/ui-consistency`  
**Worktree:** `.worktrees/ui-consistency`

## Changes

### Quick status (dashboard)

- Dropped forced `dark:border-slate-700 dark:bg-slate-900` on Quick status — normal theme Card (white / dark panel).
- Internet: `text-emerald-600 dark:text-emerald-400` / `text-red-600 dark:text-red-400`.
- VPN On: emerald light+dark pair; VPN Off: `text-gray-500 dark:text-gray-400` (no slate-on-white leftover).
- Monitor icon: `text-gray-500 dark:text-gray-400`.

### Logs level badge

- Kept custom dense uppercase spans — `ui/Badge` is rounded-full pastel; not visual parity with terminal chips.
- `LOG_LEVEL_COLORS` + fallback: dark pairs; `notice` uses emerald (was green).

### Shell titles

Convention (unchanged; documented in `router.tsx` comment + here):

| Route area | Shell title |
|------------|-------------|
| Top-level pages | Flat (`Dashboard`, `Clients`, `VPN`, `Services`, `System`, `Logs`) |
| WiFi sub-routes (`/wifi`, `/wifi/advanced`) | Flat `WiFi` (in-page tabs) |
| Network sub-routes (`/network`, `/configuration`, `/advanced`) | Flat `Network` (in-page tabs) |
| Service children | Breadcrumb `Services / Tailscale`, `Services / SQM`, `Services / Speedtest` |

Header `h1` already single style: `text-lg font-semibold text-gray-900 dark:text-white`.

### Success/fail status dots → emerald/red + dark

- `header-router-status`, `network-chart`, `interface-traffic-charts`, `uptime-log-card`, `tailscale-peer-row`, `ddns-status-panel`, `wan-status-card`.
- Offline/inactive dots: `bg-gray-300/400` + `dark:bg-gray-600`.
- Glow shadows retinted to emerald `rgba(16,185,129,0.6)`.

### Preserved

- Network Status **Connected Clients** table untouched (`network-page-status-panel.tsx`).
- Did not touch `mockServiceWorker.js`.
- Did not rewrite gray helper text beyond status chrome / titles / Quick status leftovers.

## Test summary

```
cd frontend && pnpm exec vitest run \
  src/pages/dashboard/__tests__/dashboard-page.test.tsx \
  src/pages/logs/__tests__/logs-page.test.tsx \
  src/pages/network/__tests__/network-page.test.tsx \
  src/components/layout/__tests__/sidebar.test.tsx
→ Test Files  4 passed (4)
→ Tests       22 passed (22)
```

Dashboard test asserts Quick status emerald/gray theme pairs for Reachable + VPN On/Off.

## Concerns

- `Badge` `success` variant still uses green (not emerald) — left alone; only custom dots aligned.
- CheckCircle / Shield icons elsewhere may still use `text-green-500` without dark pairs — out of Task 9 dot scope.
- SourceCards remain forced-dark slate chrome (Task 3); Quick status deliberately theme-aware.
