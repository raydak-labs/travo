# Resilience & Code Quality Improvements

> **Status:** Implemented (2026-03-24)
> **Scope:** Bug fixes, goroutine safety, frontend maintainability, test coverage

This document records the analysis, rationale, and implementation details for the code quality and resilience improvements added to the travel router codebase. All items were identified via systematic codebase review.

---

## Bug 1: Backup/Restore Always Sends Empty Auth Token (401 Failures)

**File:** `frontend/src/hooks/use-system.ts`
**Severity:** High — all backup/restore operations silently failed with 401 Unauthorized.

### Root Cause

`useBackup()` (line 178) and `useRestore()` (line 207) used `localStorage.getItem('token')` but the correct storage key is `'openwrt-auth-token'`. This discrepancy existed because `useFirmwareUpgrade()` already used the correct key, but the backup/restore mutations were added later with the wrong key.

### Fix

Replace `'token'` with `'openwrt-auth-token'` (and `sessionStorage` fallback key) in both `useBackup` and `useRestore`.

---

## Bug 2: WebSocket Hub Leaks Dead Connections Forever

**File:** `backend/internal/ws/hub.go`
**Severity:** Medium — causes CPU waste and log spam on every broadcast tick.

### Root Cause

`Broadcast()` held `RLock` and called `conn.Close()` on write failure, but never removed the failed connection from the `clients` map. On every subsequent 2-second broadcast, the dead connection was retried, failed again, and `Close()` was called again — indefinitely.

### Fix

Collect failed connections during the `RLock` iteration (read phase), then upgrade to write lock after the loop to delete them and close properly:

```go
func (h *Hub) Broadcast(data []byte) {
    h.mu.RLock()
    var failed []*websocket.Conn
    for conn := range h.clients {
        if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
            failed = append(failed, conn)
        }
    }
    h.mu.RUnlock()
    if len(failed) > 0 {
        h.mu.Lock()
        for _, conn := range failed {
            delete(h.clients, conn)
            conn.Close()
        }
        h.mu.Unlock()
    }
}
```

---

## Bug 3: Hub.Stop() Has No sync.Once — Double-Close Panics

**File:** `backend/internal/ws/hub.go`
**Severity:** Medium — can cause server panic on shutdown.

### Root Cause

`Hub.Stop()` called `close(h.stopCh)` with no guard. `BandSwitchingService` and `TokenBlocklist` both already had this fixed with `sync.Once`, but `Hub` did not.

### Fix

Add `stopOnce sync.Once` field to `Hub` struct and wrap the close in `stopOnce.Do(...)`.

---

## Bug 4: AlertService.Start() Creates Duplicate Goroutines

**File:** `backend/internal/services/alert_service.go`
**Severity:** Low-Medium — double alerts, double checks, harder to stop cleanly.

### Root Cause

`Start()` launched a goroutine unconditionally. If called twice (possible during testing or future refactoring), two goroutines would compete to read from the same `stopCh` and emit duplicate alerts.

### Fix

Add `startOnce sync.Once` field and wrap goroutine launch in `startOnce.Do(...)`.

---

## Improvement 1: Add Panic Recovery to Goroutines

**Files:** `hub.go`, `alert_service.go`, `uptime_tracker.go`, `band_switching_service.go`
**Severity:** Low — but important on constrained hardware where panics kill background monitoring indefinitely.

### Rationale

On a travel router running ath11k/IPQ6018, a nil pointer or unexpected value in system stats collection kills the goroutine permanently (no supervisor will restart it). The service continues running but all background monitoring stops silently.

### Fix

Add `defer func() { if r := recover(); r != nil { log.Printf(...) } }()` at the top of each long-running goroutine. Applied to:

- `ws/hub.go` — `Start()` goroutine
- `services/alert_service.go` — `Start()` goroutine
- `services/uptime_tracker.go` — `Start()` goroutine
- `services/band_switching_service.go` — `Start()` goroutine

---

## Improvement 2: Add tsc --noEmit to Makefile

**File:** `Makefile`

### Rationale

`make lint` only ran ESLint and `go vet`. TypeScript type errors were not caught in the lint pipeline. Silent type errors could ship to production.

### Fix

Added `cd frontend && pnpm tsc --noEmit` to the `lint` target in `Makefile`.

---

## Improvement 3: network-page.tsx Refactoring (984 Lines)

**File:** `frontend/src/pages/network/network-page.tsx`

### Rationale

Same issue as `wifi-page.tsx` (previously 889 lines, refactored to 568 in a prior session). The network page at 984 lines had three large self-contained sections (DHCP config, DNS config, DDNS config) mixed inline with page-level state.

### What Was Extracted

| New File | Content | Hooks Owned |
|----------|---------|-------------|
| `dhcp-config-card.tsx` | DHCP start/limit/lease config form | `useDHCPConfig`, `useSetDHCPConfig` |
| `dns-config-card.tsx` | Custom DNS toggle + primary/secondary servers | `useDNSConfig`, `useSetDNSConfig` |
| `ddns-card.tsx` | DDNS provider, domain, credentials, status | `useDDNSConfig`, `useDDNSStatus`, `useSetDDNSConfig` |

**Result:** 984 → 648 lines in `network-page.tsx`. Each extracted component owns its hooks and local state.

**Not extracted:** `ClientsTable`, `DataUsageSection`, `USBTetheringSection`, `InterfaceTrafficCharts` — already separate files.

---

## Gap 1: AP Health Check Has No Tests

**File:** `backend/internal/services/ap_health.go` → new `ap_health_test.go`

### Rationale

`EnsureAPRunning()` is critical startup logic that repairs AP WiFi config. If it breaks, the router may fail to broadcast any WiFi network on boot. Despite being critical, it had no test coverage.

### Tests Added (`ap_health_test.go`)

1. `TestEnsureAPRunning_HealthyConfig` — valid config produces no changes
2. `TestEnsureAPRunning_MissingSSID` — missing SSID is repaired with band-specific default
3. `TestEnsureAPRunning_DisabledAPWithActiveSTA_Skipped` — disabled AP on radio with STA is not re-enabled (ath11k crash guard)
4. `TestEnsureAPRunning_DisabledAPWithoutSTA_ReEnabled` — disabled AP on radio without STA is re-enabled
5. `TestEnsureAPRunning_MissingKey_WhenEncryptionSet` — missing WPA key is set to default
6. `TestEnsureAPRunning_MissingCountry_Fixed` — missing radio country defaults to US

---

## Gap 2: Hub Tests

**File:** `backend/internal/ws/hub_test.go`

### Tests Added

- `TestHubStopIdempotent` — verifies `Stop()` called twice does not panic (validates `sync.Once` fix)
- `TestHubRegisterUnregister` — verifies `ClientCount()` returns 0 initially
