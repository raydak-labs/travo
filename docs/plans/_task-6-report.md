# Task 6 Report — EmptyState adoption

**Status:** done
**Commit hash:**     
**Commit:** `refactor(ui): use EmptyState for empty content`  
**Branch:** `fix/ui-consistency`

## Files changed

- `frontend/src/pages/clients/clients-connected-table.tsx`
- `frontend/src/components/clients/clients-dhcp-reservations-card.tsx`
- `frontend/src/pages/logs/logs-text-view.tsx`
- `frontend/src/pages/system/ssh-keys-list.tsx`
- `frontend/src/pages/setup/setup-wifi-network-list.tsx`
- `frontend/src/pages/wifi/wifi-scan-list.tsx`
- `frontend/src/pages/wifi/mac-policy-table.tsx`
- `frontend/src/pages/wifi/mac-address-card.tsx`
- `frontend/src/pages/network/wan-config-card.tsx`
- `frontend/src/pages/network/ipv6-card.tsx`
- `frontend/src/pages/dashboard/dashboard-page.tsx`
- `frontend/src/components/layout/header-notifications-menu.tsx`
- `docs/plans/2026-07-20-ui-consistency.md` (Task 6 checkboxes)
- `docs/plans/_task-6-report.md`

## What changed

- Replaced one-off empty `<p>` / `<span>` / div copy with `<EmptyState message="…" />`.
- Logs empty path: render `EmptyState` instead of gray text inside the terminal `<pre>` (pre only when lines exist).
- Left loading (`Skeleton`) and `InlineError` untouched.
- Did not migrate install/CTA prompts (Tailscale/WireGuard), chart “Waiting…/Collecting…” placeholders, or form labels.
- Did not touch `mockServiceWorker.js`.

## Tests

- `cd frontend && pnpm test -- --run` (clients, logs, system, wifi, setup, network, dashboard, clients/layout components): **48 files / 262 tests passed**
- No existing tests asserted old empty markup; copy strings preserved so assertions by text still match.

## Concerns

- Logs empty no longer mounts `data-testid="log-content"` (only when lines present). Mock data keeps current logs test green; empty-log tests would need EmptyState text query.
- Dashboard SourceCard Ethernet empty now uses shared EmptyState (gray theme tokens) on forced-dark SourceCard — contrast OK, visual weight slightly different (`py-6` center).
- Parallel Label migration edits were present in worktree; **not** included in this commit.
- Concurrent agents reset branch during earlier commit attempts; this commit re-applied EmptyState-only paths.
