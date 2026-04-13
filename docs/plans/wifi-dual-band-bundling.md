---
title: "Plan: WiFi dual-band bundling & automatic band switching"
description: "Planning / design notes: Plan: WiFi dual-band bundling & automatic band switching"
updated: 2026-04-13
tags: [plan, traceability, wifi]
---

# Plan: WiFi dual-band bundling & automatic band switching

**Status:** Not implemented (documented for future work)
**Related requirements:** [1.1 Upstream WiFi — Dual-band scan bundling](../requirements/tasks_done.md#wifi-and-network-foundation)

---

## Goal

1. When the same SSID is broadcast on both 2.4 GHz and 5 GHz, show **one network** in the scan list (macOS-style) instead of two separate rows.
2. Let users pick a band when connecting to a dual-band network.
3. **Automatically switch bands** based on signal quality — prefer 5 GHz when strong, fall back to 2.4 GHz when weak, and switch back when 5 GHz recovers. Thresholds and timings are user-configurable.

---

## Background

- **Current behaviour:** Scan returns one result per (SSID, BSSID, channel) from each radio. "MyHome" on 2.4 GHz and "MyHome" on 5 GHz appear as two separate rows.
- **User expectation:** A single "MyHome" entry showing "2.4 GHz, 5 GHz" or "Dual-band".
- **OpenWrt constraint:** One STA interface (`sta0`) bound to one radio at a time. Switching bands = changing `wireless.sta0.device` from one radio to the other + UCI apply/confirm. SSID and password stay unchanged.
- **Verified on device (AXT1800/IPQ6018):**
  - Cross-radio scanning works: while connected on 5 GHz (`phy0-sta0`), we can scan the 2.4 GHz radio (`phy1-ap0`) to check if the same SSID is available and its signal strength.
  - Band switch = `uci set wireless.sta0.device=radioX` + apply/confirm. Brief connectivity gap (~2-5 seconds).

---

## Part 1: Scan list grouping

### A. Frontend-only grouping (recommended first step)

- **API:** Unchanged. Returns flat `WifiScanResult[]`.
- **Frontend:** Group results by **(SSID, encryption)** — mandatory same encryption to avoid merging unrelated networks (e.g. "FreeWiFi" open vs. "FreeWiFi" WPA2 at airports).
- **Per-group row displays:**
  - SSID
  - Band badges: "2.4 GHz", "5 GHz" (or both if dual-band)
  - **Per-band signal indicators** (not just "best signal") — e.g. `5 GHz ▪▪▪▪ -42 dBm | 2.4 GHz ▪▪▪ -58 dBm`
  - If connected: **which band is active** — e.g. "Connected (5 GHz)"
- **Expand/collapse** (optional): Power users can expand a grouped row to see individual APs (BSSID, channel, exact signal). Default: collapsed.
- **Connect:** Tapping Connect opens the password dialog, which additionally shows a **band picker** when both bands are available (see Part 2).

**Pros:** No API/backend change; quick to ship.
**Cons:** Slight duplication of grouping logic if we later add a grouped API.

### B. Backend-grouped scan API (optional follow-up)

- **API:** New response shape with "network groups":
  - `ssid`, `encryption`
  - `aps`: array of `{ bssid, channel, band, signal_dbm }`
- **Pros:** Single source of truth; one grouping logic.
  **Cons:** API change; shared types and frontend need updates.

---

## Part 2: Band selection on connect

When connecting to a dual-band network, the connect dialog shows:

- Password input (as today)
- **Band picker** with smart default:
  - Default to **5 GHz** if its signal is above -70 dBm
  - Default to **2.4 GHz** if 5 GHz is below -70 dBm or unavailable
  - Display signal strength per option: `● 5 GHz (-42 dBm, strong)  ○ 2.4 GHz (-58 dBm, good)`
- User can override the default before connecting

**Backend change:** Extend `WifiConnect` to accept an optional `band` parameter. When provided, the backend attaches `sta0` to the matching radio before applying.

---

## Part 3: Automatic band switching

### Concept

A background monitor periodically checks signal quality of the active STA connection. When signal degrades below a configurable threshold for a sustained period, it switches to the alternate band (if the same SSID is available there with better signal). It also switches back when the preferred band recovers.

### How it works

1. **New goroutine** in `WifiService` (same pattern as `AlertService` ticker — configurable interval, default 10s).
2. Each tick:
   - Read current STA signal via `iwinfo <sta-iface> info` (fast, no scan needed).
   - If signal is **below the down-switch threshold** → increment a "weak signal" counter.
   - If signal is **above the down-switch threshold** → reset the counter.
   - When counter reaches `downSwitchChecks` (= configured delay / check interval) → **trigger band switch**.
3. **Band switch procedure:**
   - Scan the alternate radio for the current SSID (via `iwinfo <other-radio-ap> scan | grep SSID`).
   - If SSID found with signal above a **minimum viable threshold** (-80 dBm):
     - Write crash guard file `/etc/openwrt-travel-gui/band-switch-in-progress`
     - `uci set wireless.sta0.device=<other-radio>`
     - UCI apply/confirm (uses existing safe rollback flow)
     - Remove crash guard on success
     - Log which band we switched to and why
   - If SSID **not found** on alternate radio → do nothing, stay on current band.
4. **Switch-back logic** (same goroutine):
   - After a down-switch, periodically scan the **preferred band** (5 GHz) from the alternate radio.
   - If preferred band signal is **above the up-switch threshold** for `upSwitchChecks` consecutive checks → switch back.
   - This prevents flapping: the preferred band must be consistently strong, not just a momentary spike.

### Configurable parameters (stored in JSON, editable in UI)

| Parameter                | Default | Description                                                                         |
| ------------------------ | ------- | ----------------------------------------------------------------------------------- |
| `enabled`                | `false` | Master toggle for auto band switching                                               |
| `preferredBand`          | `5g`    | Which band to prefer when signal is adequate                                        |
| `checkIntervalSec`       | `10`    | How often to check signal (seconds)                                                 |
| `downSwitchThresholdDBm` | `-70`   | Signal below this triggers switch-away timer                                        |
| `downSwitchDelaySec`     | `30`    | Signal must be weak for this long before switching (3 checks at 10s)                |
| `upSwitchThresholdDBm`   | `-60`   | Preferred band must be above this to switch back                                    |
| `upSwitchDelaySec`       | `60`    | Preferred band must be strong for this long before switching back (6 checks at 10s) |
| `minViableSignalDBm`     | `-80`   | Don't switch to a band weaker than this                                             |

**Asymmetric hysteresis by design:** Switching away is faster (30s default) because the user is losing connectivity. Switching back is slower (60s default) to avoid flapping when walking back and forth near the range boundary.

### Config file

`/etc/openwrt-travel-gui/band-switching.json`:
```json
{
  "enabled": false,
  "preferred_band": "5g",
  "check_interval_sec": 10,
  "down_switch_threshold_dbm": -70,
  "down_switch_delay_sec": 30,
  "up_switch_threshold_dbm": -60,
  "up_switch_delay_sec": 60,
  "min_viable_signal_dbm": -80
}
```

### Safety

- **Crash guard:** `/etc/openwrt-travel-gui/band-switch-in-progress` — prevents retry loop on reboot.
- **UCI rollback:** Uses `uci apply` with 30s rollback timeout. If the device crashes during switch, old config is restored on boot.
- **Rate limiting:** After any switch, impose a cooldown (2 minutes) before the next switch to avoid rapid flapping.
- **Only when connected:** Monitor only runs when STA has an active connection. If disconnected, auto-reconnect (existing feature) handles reconnection first.
- **wifi up avoidance:** Uses UCI apply/confirm only (no `wifi up` from Go code), matching the existing safe pattern.

### UI (WiFi settings section)

- **Toggle:** "Auto band switching" (on/off)
- **Basic mode (default):** Toggle only. Uses sensible defaults.
- **Advanced mode (expandable):** Shows all configurable thresholds:
  - Preferred band (dropdown: 5 GHz / 2.4 GHz)
  - "Switch away when signal below __ dBm for __ seconds"
  - "Switch back when preferred band above __ dBm for __ seconds"
  - Check interval (slider or number input)
- **Status indicator:** When active, show current monitoring state in the WiFi status area:
  - "Auto-switching: monitoring (5 GHz, -42 dBm)"
  - "Auto-switching: weak signal (12s / 30s before switch)"
  - "Auto-switching: switched to 2.4 GHz (5 GHz too weak)"

### API endpoints

- `GET /api/v1/wifi/band-switching` — Get current config + monitoring status
- `PUT /api/v1/wifi/band-switching` — Update config (validates, restarts monitor if needed)

---

## Part 4: Smart initial band preference (on connect)

When creating/moving the STA for a new connection:
- If user didn't pick a band explicitly (single-band network or legacy connect), prefer **5 GHz** when its signal is within 15 dBm of 2.4 GHz signal, otherwise default to 2.4 GHz.
- This accounts for 5 GHz degrading faster through walls — a fixed "always prefer 5 GHz" is too blunt for a travel router.

---

## Recommended implementation order

1. **Frontend scan grouping** (Part 1A) — Group by `(SSID, encryption)`, per-band signal badges, connected-band indicator, optional expand/collapse.
2. **Band picker on connect** (Part 2) — Band selection in connect dialog with smart default. Backend `Connect` accepts optional `band` parameter.
3. **Automatic band switching** (Part 3) — Background monitor goroutine, configurable thresholds, UI settings panel.
4. **Smart initial band preference** (Part 4) — Signal-aware default band on first connect.
5. **Optional — Backend-grouped API** (Part 1B) — If we want a single source of truth for grouping logic.

---

## Out of scope

- **Multi-STA simultaneous connection:** Connecting to same SSID on both bands simultaneously. Would require two STA interfaces and bonding/load-balancing — complex and not standard on OpenWrt.
- **802.11r/k/v roaming:** Driver/AP-level fast roaming. Requires AP support and is not controllable from the STA side.

---

## References

- Traceability: [tasks_done.md](../requirements/tasks_done.md#wifi-and-network-foundation) (dual-band shipped)
- Current scan: backend `WifiService.Scan()` returns flat list from all radios; frontend `WifiScanList` renders one row per result.
- Connect flow: `WifiConnectDialog` passes `network.ssid` and password; backend `Connect()` sets STA SSID/key and applies; band is determined by which radio the STA is on.
- Auto-reconnect pattern: `/etc/openwrt-travel-gui/autoreconnect.json` + cron script + crash guard.
- Alert service pattern: `AlertService` goroutine with `time.NewTicker` for periodic checks.
- UCI apply/confirm: `services/uci_apply.go` — safe rollback-based wireless apply.
- Verified on device: AXT1800 (IPQ6018) supports cross-radio scanning while STA is connected; band switch via UCI device change works.
