---
title: "ADR 0002: Wireless model, health, and LuCI-style UCI apply"
status: Accepted
date: 2026-05-14
tags: [adr, wireless, wwan, repeater, uci, rpcd, openwrt]
---

# ADR 0002: Wireless model, health, and LuCI-style UCI apply

## Status

Accepted.

## Context

Travel-router behavior depends on predictable **STA/WWAN**, **repeater** radio layout, and safe application of `wireless` (and related) UCI. OpenWrt’s **LuCI** uses rpcd **`uci apply`** with rollback and explicit **`uci confirm`** so a bad wireless change can time out back to the previous config. ath11k/IPQ6018 hardware is sensitive to **`wifi reload`**. Travo must align with these constraints while exposing **health** and **reconcile** actions to the UI.

## Decision

### 1. STA / WWAN ownership

- At most **one enabled** `wifi-iface` with `mode=sta` bound to **`network=wwan`**.
- Saved upstream networks are ordered and persisted (`wifi-priorities.json` under `/etc/travo/`); only one profile is active at runtime where the product model requires it.
- Any code path that creates or enables a STA for upstream use must **`ensureWwanNetwork`** (or equivalent): `network=wwan` exists and participates in the **`wan` firewall zone**. Violations are **errors**, not soft warnings.
- **Validation** before staging apply: `validateWirelessConsistency` rejects multiple active WWAN STAs (`ErrMultipleActiveSTA`).

### 2. Repeater and multi-radio layout

- With **two or more radios**, repeater mode prefers **STA and downlink AP on different radios** to avoid same-channel coupling and upstream-driven local AP failure on ath11k.
- **`allow_ap_on_sta_radio`** in `repeater-options.json` (`/etc/travo/repeater-options.json`) is the explicit escape hatch for same-radio STA+AP.
- **Repeater reconciliation** is a first-class API concern: changing AP or repeater options must not silently leave a fragile layout when a safer split exists.
- **Atomic reconcile rule (MUST):** Any function that activates a STA or moves a STA to a different radio **must** call `reconcileRepeaterAPRadioLayout()` *before* `uci.Commit("wireless")` in the same call. Committing an AP+STA-on-same-radio state even transiently is enough to crash ath11k/IPQ6018. Staging the STA change, reconciling (which stages AP enable/disable changes), and then committing once is the required pattern. The health-check banner and its "Fix" button are a fallback only — they must never be the primary path for avoiding a crash.

### 3. AP credential model

- Default UX: **one shared SSID/password** across enabled downlink AP sections; per-radio enable remains visible; optional per-radio overrides may exist behind UI toggles.

### 4. Health and recovery API

- **`GET /api/v1/wifi/health`** reports invariant violations (e.g. fragile repeater layout) and drives warnings plus **reconcile** actions in the frontend.
- **Auto-reconnect** scripts use a **failure counter** and crash-guard file under `/etc/travo/` (see ADR 0003) so broken credentials are not retried forever.

### 5. LuCI-style apply / confirm (user-driven wireless changes)

- **`RealUCIApplyConfirm`** (`backend/internal/services/uci_apply.go`) implements rpcd session login, copies **`/etc/config/{wireless,network,system,firewall,dhcp}`** into the session tree, calls **`uci apply`** with **`rollback: true`** and **30s** timeout, then **`uci confirm`** only when invoked after success.
- **`WifiService.stageWirelessApply`**: validates consistency → **`StartApply(uciApplyConfigs)`** → returns a **token** (session id) and rollback timeout for the client.
- **`WifiService.ConfirmApply(token)`** calls **`Confirm`** after the browser proves reachability. The backend **must not** self-confirm immediately after `StartApply` without that proof (see `docs/architecture.md` §3).

### 6. Scripts, packaging, and `wifi` commands

- **User-facing** wireless mutations go through the apply/confirm path above when `applier` is configured; they **must not** run **`wifi`**, **`wifi up`**, or **`wifi reload`** as part of apply (matches `docs/architecture.md` §3).
- **Install / uci-defaults** flows write UCI only; the operator applies via LuCI **Save & Apply** or reboot.
- **`applyWireless`** may use **`ApplyAndConfirm`** for **internal, synchronous** guarded paths when `applier` is set; when `applier` is nil (e.g. some tests), a **`Reloader`** path may exist—production device wiring uses the real applier.

## Consequences

- New wireless features that touch `network`/`firewall`/`dhcp` must consider whether copied config set in **`uciApplyConfigs`** needs extending so rollback snapshots stay consistent.
- Same-radio repeater remains supported but must stay **opt-in** and **visible** in health API responses.

## References

- `backend/internal/services/wifi_service.go` — invariants, `stageWirelessApply`, `ConfirmApply`, `uciApplyConfigs`
- `backend/internal/services/uci_apply.go` — rpcd apply/confirm
- `docs/architecture.md` §2–3
