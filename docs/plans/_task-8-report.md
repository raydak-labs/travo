# Task 8 Report — Form Label migration

**Status:** DONE  
**Commit:** `refactor(ui): migrate form labels to Label primitive` (tip of `fix/ui-consistency`)  
**Branch:** `fix/ui-consistency`  
**Worktree:** `/Users/marbaced/projects/raydak/travo/.worktrees/ui-consistency`

## Changes

Migrated dense form field `<label>` elements to shared `Label` (`text-xs font-medium text-gray-500 dark:text-gray-400`).

**Files migrated: 29**

| Area | Files |
|------|-------|
| Network | `dns-entries-card`, `wol-card`, `dhcp-reservation-add-form`, `firewall-port-forward-add-form-grid`, `lan-dns-server-fields`, `dhcp-pool-form-fields`, `ddns-enabled-fields`, `failover-card` |
| System | `system-timezone-card`, `ntp-config-server-fields`, `ntp-config-edit-form`, `led-schedule-form`, `firmware-upgrade-form-fields` |
| WiFi | `mac-address-clone-block`, `ap-unified-config-form`, `ap-radio-form-credentials-and-actions`, `guest-wifi-enabled-fields`, `wifi-schedule-form-fields`, `wifi-hidden-network-dialog-fields`, `confirm-radio-disable-dialog`, `repeater-wizard/configure-ap-step`, `repeater-wizard/select-upstream-step` |
| VPN | `split-tunnel-card` (CIDR field only) |
| Auth/Setup | `login-form-fields`, `ap-step-credentials-fields`, `password-step-form-fields`, `wifi-step-password-field` |
| Services | `sqm-section` |
| Primitives | `components/ui/input.tsx` (`label` prop → `Label`) |

~68 `<Label>` call sites. Login/setup/confirm/SQM/failover keep prominence via `className` overrides (`text-sm …`). Dense wifi/network labels drop redundant `text-xs`/`text-gray-600` (contract `text-gray-500`).

## Skipped (intentional)

- Radio/checkbox wrappers with `cursor-pointer` (`split-tunnel` modes, `wifi-connect-dialog` band picker)
- `switch.tsx` primitive wrapper
- Helper `<p>` / section headings / table chrome (not field labels)
- EmptyState parallel edits in other files (not staged)
- `frontend/public/mockServiceWorker.js`

## Tests

`54e724e1969118c885285e694f827870d861fd36``text
pnpm exec vitest run \
  src/components/ui/__tests__/label.test.tsx \
  src/pages/login/__tests__/login-page.test.tsx \
  src/pages/setup/__tests__/ap-step.test.tsx \
  src/pages/wifi/__tests__/wifi-connect-dialog.test.tsx \
  src/components/wifi/__tests__/repeater-wizard.test.tsx \
  src/pages/services/__tests__/sqm-page.test.tsx \
  src/pages/vpn/__tests__/vpn-page.test.tsx \
  src/pages/network/__tests__ \
  src/pages/system/__tests__ \
  src/pages/wifi/__tests__
→ 12 files, 58 tests passed
`54e724e1969118c885285e694f827870d861fd36``

No test updates required (queries by accessible name still resolve).

## Concerns

- Dense labels that used `text-gray-600` now contract `text-gray-500` (intentional).
- `Input` built-in `label` prop now renders `Label` with login-style `text-sm` override — wireguard/alert thresholds pick this up automatically.
- Full `make lint` / `make test` / `make build` deferred to Task 10.
