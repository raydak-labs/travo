---
title: "Plan: Data Usage Tracking (via vnstat)"
description: "Planning / design notes: Plan: Data Usage Tracking (via vnstat)"
updated: 2026-04-13
tags: [metrics, plan, traceability]
---

# Plan: Data Usage Tracking (via vnstat)

**Status:** Not implemented
**Priority:** Medium
**Related requirements:** [2.4 Data Usage Tracking](../requirements/tasks_done.md#wifi-and-network-foundation)

---

## Goal

Track cumulative network data usage per WAN source (Ethernet, WiFi/WWAN, USB Tether) with persistent counters across reboots. Provide data budgets with warning thresholds and reset options.

---

## Approach: vnstat as Optional Service

`vnstat` is a lightweight (~60KB) network traffic monitor that stores historical data in a SQLite database. It's purpose-built for persistent traffic accounting and is available in OpenWRT packages.

**Key decision:** vnstat is installed as an **optional service** via the Services page (default: not installed). Data usage features are greyed out / hidden until vnstat is installed.

---

## Phases

### Phase 1 — vnstat as Installable Service

1. Add `vnstat` to the service registry (like AdGuard, WireGuard, Tailscale)
2. Package: `vnstat2` (OpenWRT 25.12 uses vnstat2)
3. After install: auto-configure monitored interfaces (`eth0`, `wwan0`, `wg0`)
4. Init script: `vnstatd` daemon runs in background, samples every 5 minutes
5. Frontend: Show "Data Usage" service card on Services page

**Files:**
- `backend/internal/services/service_registry.go` — add vnstat service definition
- `backend/internal/services/data_usage_service.go` — new service
- `frontend/src/pages/services/` — vnstat card

### Phase 2 — Data Usage API

**Endpoints:**
- `GET /api/v1/network/data-usage` — current period usage per interface
- `GET /api/v1/network/data-usage/history?interface=wwan0&period=daily` — historical data
- `POST /api/v1/network/data-usage/reset` — reset counters

**Backend implementation:**
- Parse `vnstat --json` output (vnstat2 has excellent JSON support)
- Map interface names to human labels (eth0 → "Ethernet WAN", wwan0 → "WiFi Uplink")
- Aggregate by day/month

**Response shape:**
```json
{
  "interfaces": [
    {
      "name": "wwan0",
      "label": "WiFi Uplink",
      "today": { "rx_bytes": 104857600, "tx_bytes": 52428800 },
      "month": { "rx_bytes": 5368709120, "tx_bytes": 1073741824 },
      "total": { "rx_bytes": 10737418240, "tx_bytes": 2147483648 }
    }
  ]
}
```

### Phase 3 — Data Budget & Alerts

**Config stored in:** `/etc/openwrt-travel-gui/data_budget.json`

```json
{
  "budgets": [
    {
      "interface": "wwan0",
      "monthly_limit_bytes": 10737418240,
      "warning_threshold_pct": 80,
      "reset_day": 1
    }
  ]
}
```

**Endpoints:**
- `GET /api/v1/network/data-usage/budget` — get budget config
- `PUT /api/v1/network/data-usage/budget` — set budget config

**Alerts:** When usage exceeds warning threshold, fire WebSocket alert via AlertService.

### Phase 4 — Dashboard Integration

- Data usage summary card on dashboard (total today, total this month)
- Progress bar showing budget usage (green → yellow → red)
- Per-WAN-source breakdown

**Files:**
- `frontend/src/pages/dashboard/data-usage-card.tsx` — new dashboard card
- `frontend/src/pages/network/data-usage-section.tsx` — detailed view in network tab
- `frontend/src/hooks/use-data-usage.ts` — React Query hooks

---

## Testing Strategy

- **Unit tests:** Mock `vnstat --json` output, test parsing and aggregation
- **Service detection:** Test that UI gracefully handles vnstat not installed
- **Budget alerts:** Test threshold crossing triggers WebSocket alert

---

## Notes

- vnstat database lives in `/var/lib/vnstat/` — survives reboots if on persistent storage
- On flash-constrained devices, the DB is small (few KB per interface per month)
- Counter reset: `vnstat --remove -i <iface> && vnstat --add -i <iface>`
- vnstat2 auto-detects new interfaces; we just need to ensure monitoring starts
