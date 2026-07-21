# Approach A report — nested cards / form chrome

**Branch:** `fix/ui-consistency`  
**Worktree:** `.worktrees/ui-consistency`  
**Commit message:** `fix(ui): flatten nested cards and align form chrome`  
**Status:** done

## Summary

- Added shared `CardInset` (`default` / `muted`) for nested regions without a second card shadow.
- Equal-height WiFi Current Connection / Saved Networks sibling cards.
- Shortened Scan / Hidden button labels; wrap button rows; full names in `aria-label` / `title`.
- Flattened nested bordered panels to `CardInset` across WiFi AP / guest / MAC / schedule.
- DNS + DHCP Add buttons default `h-10`; Diagnostics tablist matches page tab-bar; Run `h-10`.
- Services `ServiceCard`: `shadow-none` so outer Installed Services card keeps elevation.

## Files changed

### New
- `frontend/src/components/ui/card-inset.tsx`
- `docs/plans/_approach-a-report.md` (this file)

### Docs
- `docs/ui-theming.md` — Nested regions → CardInset

### WiFi equal-height + Scan/Hidden
- `frontend/src/pages/wifi/wifi-wireless-panel.tsx`
- `frontend/src/pages/wifi/wifi-current-connection-card.tsx`
- `frontend/src/pages/wifi/wifi-saved-networks-card.tsx`
- `frontend/src/pages/wifi/wifi-scan-dialog.tsx`
- `frontend/src/pages/wifi/wifi-hidden-network-dialog.tsx`
- `frontend/src/pages/wifi/__tests__/wifi-page.test.tsx`

### Nested panels → CardInset
- `frontend/src/pages/wifi/ap-config-card.tsx`
- `frontend/src/pages/wifi/ap-radio-section.tsx`
- `frontend/src/pages/wifi/ap-unified-config-form.tsx`
- `frontend/src/pages/wifi/guest-wifi-enabled-fields.tsx`
- `frontend/src/pages/wifi/mac-policy-add-form.tsx`
- `frontend/src/pages/wifi/wifi-schedule-form-fields.tsx`

### DNS / Diagnostics / Services
- `frontend/src/pages/network/dns-entries-card.tsx`
- `frontend/src/pages/network/dhcp-reservation-add-form.tsx`
- `frontend/src/pages/network/diagnostics-card.tsx`
- `frontend/src/pages/services/service-card.tsx`

## Tests

`cd frontend && pnpm test` (via root `make test` path): **48 files / 262 tests passed**.

Also: shared 58 passed; backend go tests ok.
