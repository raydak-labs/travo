# Plan: WiFi dual-band scan bundling

**Status:** Not implemented (documented for future work)  
**Related requirements:** [1.1 Upstream WiFi — Dual-band scan bundling](../requirements/requirements.md#11-upstream-wifi-sta--wwan--connect-to-existing-wifi)

---

## Goal

When the same SSID is broadcast on both 2.4 GHz and 5 GHz, show **one network** in the scan list (like macOS) instead of two separate rows. Optionally prefer 5 GHz when creating/using the STA for connect.

---

## Background

- **Current behaviour:** Scan returns one result per (SSID, BSSID, channel) from each radio. So “MyHome” on 2.4 GHz and “MyHome” on 5 GHz appear as two entries.
- **User expectation:** Many users are used to a single “MyHome” entry that may list “2.4 GHz, 5 GHz” or “Dual-band”.
- **OpenWrt constraint:** We have one STA interface bound to one radio, so we connect to one band at a time. There is no automatic band switching (5G ↔ 2.4G) with a single STA; that would require multiple STAs or driver/AP features (e.g. 802.11r/k/v).

---

## Options

### A. Frontend-only grouping (recommended first step)

- **API:** Unchanged. Continues to return flat `WifiScanResult[]`.
- **Frontend:** Group results by SSID (and optionally same encryption). Render one row per group with:
  - SSID
  - Band badges: “2.4 GHz”, “5 GHz” (or “Dual-band”)
  - Best signal among the group (e.g. strongest dBm)
- **Connect:** When user taps Connect on that row, pass SSID + password (derive from any item in the group). Backend already uses only SSID/password.

**Pros:** No API/backend change; quick to ship.  
**Cons:** Slight duplication of grouping logic if we later add a grouped API.

### B. Backend-grouped scan API

- **API:** New response shape, e.g. list of “network groups”:
  - `ssid`, `encryption`
  - `aps`: array of `{ bssid, channel, band, signal_dbm }`
- **Frontend:** Consumes grouped list; one row per group; Connect sends SSID + password (and optionally chosen band/BSSID if we extend Connect later).

**Pros:** Clear contract; one place for grouping logic.  
**Cons:** API change; shared types and frontend need updates.

### C. Prefer 5 GHz for STA (optional, backend)

- When creating the STA (e.g. in `ensureSTASectionForScan`), if multiple radios exist, attach the STA to the **5 GHz** radio so that “Connect” to a dual-band SSID tends to use 5 GHz by default.
- Does not add automatic band switching; only sets default band for new STA.

---

## Recommended implementation order

1. **Frontend-only grouping (Option A):** Group scan results by SSID in the WiFi scan list UI; show one row per SSID with band badges and best signal; Connect unchanged (SSID + password).
2. **Optional — Prefer 5G for STA (Option C):** When creating the STA, prefer attaching it to the 5 GHz radio when available.
3. **Optional — Grouped API (Option B):** If we want a single source of truth for “one network, multiple bands”, add a grouped scan endpoint and switch the frontend to it.

---

## Out of scope (for this plan)

- **Automatic band switching:** With one STA we stay on one band. Roaming 5G ↔ 2.4G would require multiple STAs or driver/AP support; not planned here.
- **Per-BSSID connect:** Backend Connect currently uses SSID + password only. Extending to “connect to this BSSID/band” could be a follow-up if we add band selection in the UI.

---

## References

- Requirements: [docs/requirements/requirements.md](../requirements/requirements.md) — § 1.1 (Dual-band scan bundling)
- Current scan: backend `WifiService.Scan()` returns flat list from all radios; frontend `WifiScanList` renders one row per result.
- Connect flow: `WifiConnectDialog` passes `network.ssid` and password; backend `Connect()` sets STA SSID/key and applies; band is determined by which radio the STA is on.
