# Repeater radio policy, unified AP UX, and health warnings

**Date:** 2026-04-11  
**Status:** Implemented (core); two optional follow-ups tracked in [requirements §1.5](../requirements/requirements.md#15-wifi-follow-up-optional).

## Goals

- Repeater mode must not silently leave every AP enabled or AP+STA stuck on one PHY when dual-radio layout is intended.
- One logical “main Wi‑Fi” AP editor by default (same SSID/credentials on all downlink APs), with an explicit toggle for per-radio separate AP config.
- Optional policy: allow a downlink AP on the same radio as the STA when the user accepts reduced performance / complexity.
- Surface same-radio AP+STA in health and offer one-click reconcile where safe.
- Fix misleading copy (“Guest Wi‑Fi” vs downlink AP; generic “Waiting for IP” titles on all health issues).

## Problems encountered

1. **Wizard vs service policy** — Setup and mode flows could enable multiple AP sections or fight `SetMode("repeater")`, which applies downlink AP layout (typically one AP on non-STA radio, STA radio AP off unless allowed).
2. **Unified AP saves** — After merging AP edits into one form, PUTs could leave AP+STA on one radio until the next full reconcile; users needed feedback and an explicit fix path.
3. **Health banner noise** — Repeater same-radio was duplicated (repeater card + health banner) and lease/DHCP messages all used the same title.
4. **Lint / tests** — Health fixtures had to disable conflicting mock radios so new `repeater_same_radio_ap_sta` did not break unrelated tests; titles needed broader string matching for lease variants.

## Decisions

| Topic | Decision |
|-------|----------|
| Default SSID policy | One SSID/password for all downlink APs by default; “Separate settings per radio” disables sync. |
| `allow_ap_on_sta_radio` | Stored in `repeater-options.json`; `SetMode(repeater)` and AP reconciliation respect it. |
| Backend reconciliation | `SetAPConfig` calls `reconcileRepeaterAPRadioLayout` before commit; shared helper `applyRepeaterDownlinkAPPolicy` with `SetMode`. |
| User-triggered fix | `POST /api/v1/wifi/repeater/reconcile` → `ReconcileRepeaterAPLayout()` (uci apply/confirm path). |
| Health | `WifiHealth.repeater_same_radio_ap_sta` + issue entry; main health banner hidden when that is the only issue (repeater UI owns the warning). |
| Copy | Repeater banner: downlink AP on uplink radio ≠ “Guest Network”; health titles derived from issue kind / message heuristics. |

## Implementation map (for agents)

| Area | Location |
|------|----------|
| Policy + reconcile | `backend/internal/services/wifi_service.go` — `applyRepeaterDownlinkAPPolicy`, `reconcileRepeaterAPRadioLayout`, `ReconcileRepeaterAPLayout`, `sameRadioRepeaterAPSTAConflict`, `SetAPConfig` |
| Health model | `backend/internal/models/wifi.go` — `WifiHealth.RepeaterSameRadioAPSTA` |
| API route | `backend/internal/api/wifi_handlers.go`, router registration, OpenAPI |
| Shared types / route | `shared/src/types/wifi.ts`, `shared/src/api/routes.ts` |
| Repeater banner / advanced card | `frontend/src/components/wifi/wifi-repeater-same-radio-banner.tsx`, `repeater-radio-layout-card.tsx` |
| Health banner titles / dedupe | `frontend/src/components/wifi/wifi-health-banner.tsx` |
| Hooks | `frontend/src/hooks/use-wifi.ts` — `useRepeaterRadioReconcile` |

## Optional follow-ups

Detailed acceptance criteria and file pointers: [requirements §1.5](../requirements/requirements.md#15-wifi-follow-up-optional).

1. Setup wizard AP step: apply unified credentials to **all** AP UCI sections on dual-radio (not only the first list entry).
2. `PUT` repeater options: when turning **off** `allow_ap_on_sta_radio` in repeater mode, optionally run the same wireless reconcile as the reconcile endpoint (or document manual reconcile).

## Verification

- `make lint`, `make test`, `make build`
- Manual: repeater mode, dual-radio — confirm health + banner, reconcile, and AP saves move STA/AP to expected radios when policy disallows same-radio AP.
