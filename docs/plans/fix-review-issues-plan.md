---
title: "Implementation Plan: Fix All Code Review Issues"
description: "Planning / design notes: Implementation Plan: Fix All Code Review Issues"
updated: 2026-04-13
tags: [maintenance, plan, traceability]
---

# Implementation Plan: Fix All Code Review Issues

**30 issues across P0/P1/P2 severity, organized into 8 phases.**

Each phase is independently testable and committable. P0 (critical) items are addressed in Phases 1–3, P1 items in Phases 3–6, P2 items in Phases 7–8.

---

## Phase 1: Real UCI/ubus Implementations + Graceful Shutdown

**Objective:** Replace the backend's mock-only fallback with real `RealUCI` (shells out to `uci` CLI) and `RealUbus` (`ubus call`) so the app actually works on OpenWRT hardware. Add graceful shutdown handling.

**Issues addressed:** #1 (No real UCI/ubus), #18 (No graceful shutdown)

**Files to create:**
- `backend/internal/uci/real.go` — `RealUCI` struct implementing `UCI` interface via `os/exec` calls to `uci get`, `uci set`, `uci commit`, etc.
- `backend/internal/uci/real_test.go` — Unit tests using a test helper that stubs `exec.Command`
- `backend/internal/ubus/real.go` — `RealUbus` struct implementing `Ubus` interface via `os/exec` call to `ubus call <path> <method> '<json>'`
- `backend/internal/ubus/real_test.go` — Unit tests with exec stubbing

**Files to modify:**
- `backend/cmd/server/main.go` — Wire `RealUCI`/`RealUbus` when `MockMode=false`; add `os/signal` handler for `SIGINT`/`SIGTERM` with `app.ShutdownWithTimeout()`
- `backend/internal/uci/uci.go` — Add `Revert(config string) error` to interface (currently in plan but missing from interface)

**Tests to write (TDD):**
- `real_test.go` for UCI: `TestRealUCIGet`, `TestRealUCISet`, `TestRealUCICommit`, `TestRealUCIGetAll`, `TestRealUCIAddSection`, `TestRealUCIDeleteSection` — each test provides a mock exec command factory
- `real_test.go` for Ubus: `TestRealUbusCall`, `TestRealUbusCallError`, `TestRealUbusCallInvalidJSON`
- `main_test.go`: `TestGracefulShutdown` — verify server stops cleanly on signal

**Acceptance criteria:**
- `go test ./internal/uci/... ./internal/ubus/...` passes
- When `MOCK_MODE=false`, server initializes `RealUCI`/`RealUbus` (verified by log output)
- When `MOCK_MODE=true`, server still uses `MockUCI`/`MockUbus`
- Server shuts down cleanly on SIGTERM within 5s

**Estimated complexity:** L

---

## Phase 2: Captive Portal Detection + WebSocket Auth

**Objective:** Implement real captive portal detection (HTTP probe + DNS check) and secure the WebSocket endpoint with JWT validation.

**Issues addressed:** #2 (Captive portal stubbed), #3 (WebSocket no auth)

**Files to modify:**
- `backend/internal/services/captive_service.go` — Replace stub with real HTTP probe to `http://detectportal.firefox.com/canonical.html` (expect `success\n`), follow redirects to detect portal URL, DNS resolution check against known IP
- `backend/internal/services/captive_service_test.go` — Update tests for real detection logic using `httptest.Server`
- `backend/internal/ws/handler.go` — Extract JWT from `?token=` query param, validate via `auth.AuthService.ValidateToken()` before upgrading connection; reject with 401 if invalid
- `backend/internal/ws/handler_test.go` (create) — Test WS auth: valid token connects, missing token rejected, expired token rejected

**Files to create:**
- `backend/internal/ws/handler_test.go`

**Tests to write (TDD):**
- `captive_service_test.go`: `TestCaptivePortalDetected` (server returns 302 redirect), `TestCaptivePortalNotDetected` (server returns `success\n`), `TestCaptivePortalDNSMismatch`, `TestCaptivePortalTimeout`
- `handler_test.go`: `TestWebSocketAuthValidToken`, `TestWebSocketAuthMissingToken`, `TestWebSocketAuthExpiredToken`

**Acceptance criteria:**
- `CaptiveService.CheckCaptivePortal()` performs a real HTTP probe and returns detected=true with portal URL when redirected
- WebSocket endpoint returns 401 for missing/invalid tokens
- WebSocket connects successfully with valid JWT in query param
- All captive + WS tests pass

**Estimated complexity:** M

---

## Phase 3: Shared Routes Fix + Frontend API Client Hardening + Input Validation

**Objective:** Fix shared route constants to match actual backend routes, use them in all frontend hooks, add 401 redirect in api-client, and add input validation in backend handlers.

**Issues addressed:** #5 (Shared routes out of sync), #7 (No 401 redirect), #8 (API_ROUTES not used), #6 (Missing input validation), #14 (UCI write errors swallowed)

**Files to modify:**
- `shared/src/api/routes.ts` — Fix `vpn.wireguard.config` → point to `/api/v1/vpn/wireguard`, add `wifi.connection: '/api/v1/wifi/connection'`, verify all routes match `backend/internal/api/router.go`
- `shared/src/__tests__/routes.test.ts` (create) — Test that all routes start with `/api/v1/`, no duplicates, all referenced paths exist
- `frontend/src/lib/api-client.ts` — On 401 response, call `clearToken()` and `window.location.href = '/login'` (except on login endpoint itself)
- `frontend/src/lib/__tests__/api-client.test.ts` — Test 401 redirect behavior
- `frontend/src/hooks/use-wifi.ts` — Replace hardcoded paths with `API_ROUTES.wifi.*`
- `frontend/src/hooks/use-network.ts` — Replace hardcoded paths with `API_ROUTES.network.*`
- `frontend/src/hooks/use-system.ts` — Replace hardcoded paths with `API_ROUTES.system.*`
- `frontend/src/hooks/use-vpn.ts` — Replace hardcoded paths with `API_ROUTES.vpn.*`
- `frontend/src/hooks/use-services.ts` — Replace hardcoded paths with `API_ROUTES.services.*`
- `frontend/src/hooks/use-captive-portal.ts` — Replace hardcoded paths with `API_ROUTES.captive.*`
- `frontend/src/pages/dashboard/system-stats-card.tsx` — Replace hardcoded path with `API_ROUTES.system.stats`
- `backend/internal/api/wifi_handlers.go` — Validate SSID non-empty on connect
- `backend/internal/api/network_handlers.go` — Validate IP format, MTU range (68–9000) on WAN config
- `backend/internal/api/vpn_handlers.go` — Validate WireGuard config fields (private key, address required)
- `backend/internal/services/network_service.go` — Propagate `uci.Set()` errors in `SetWanConfig()` instead of ignoring them with `_ =`
- `backend/internal/services/vpn_service.go` — Propagate `uci.Set()` errors in `SetWireguardConfig()`

**Tests to write (TDD):**
- `routes.test.ts`: Route structure validation
- `api-client.test.ts`: `test401RedirectsToLogin`, `testLoginEndpointNo401Redirect`
- `wifi_handlers_test.go`: `TestConnectEmptySSID` returns 400
- `network_handlers_test.go`: `TestSetWanInvalidIP` returns 400, `TestSetWanMTUOutOfRange` returns 400
- `vpn_handlers_test.go`: `TestSetWireguardMissingPrivateKey` returns 400

**Acceptance criteria:**
- `API_ROUTES` in shared match 1:1 with backend router registration
- All frontend hooks import and use `API_ROUTES` instead of string literals
- 401 responses trigger redirect to `/login` (except on login endpoint)
- Empty SSID, invalid IP, out-of-range MTU, missing WG private key all return 400
- UCI write errors are returned to callers

**Estimated complexity:** M

---

## Phase 4: Dashboard Quick Actions + Frontend Data Fixes

**Objective:** Wire up dashboard quick actions to real API calls, fix hardcoded uptime, deduplicate utility functions, fix WifiScan auto-scan, and fix useToggleWireguard missing enable state.

**Issues addressed:** #4 (Dashboard Quick Actions TODO), #9 (Hardcoded uptime), #10 (Duplicated utils), #19 (useToggleWireguard no enable state), #20 (WifiScan enabled:false)

**Files to modify:**
- `frontend/src/pages/dashboard/quick-actions.tsx` — Import hooks (`useWifiDisconnect`+`useWifiConnect` for restart, `useToggleWireguard` for VPN toggle, add reboot mutation calling `POST /api/v1/system/reboot`)
- `frontend/src/pages/dashboard/system-stats-card.tsx` — Remove local `formatUptime`/`formatBytes` definitions, import from `@/lib/utils`; replace `formatUptime(86432)` with `formatUptime(stats.uptime_seconds)` (wired to API data)
- `frontend/src/lib/utils.ts` — Keep as single source of truth (already correct)
- `frontend/src/hooks/use-vpn.ts` — Change `useToggleWireguard` to accept `{ enable: boolean }` and pass it in POST body
- `frontend/src/hooks/use-wifi.ts` — Change `useWifiScan` to `enabled: true` with a reasonable `refetchInterval` or trigger on page mount
- `backend/internal/api/system_handlers.go` — Add `RebootHandler` for `POST /api/v1/system/reboot`
- `backend/internal/api/router.go` — Register `POST /system/reboot` route
- `backend/internal/services/system_service.go` — Add `Reboot()` method (exec `reboot` or mock)

**Files to create:**
- `frontend/src/pages/dashboard/__tests__/quick-actions.test.tsx` — Test all three quick action buttons trigger correct mutations

**Tests to write (TDD):**
- `quick-actions.test.tsx`: `TestRestartWifiCallsAPI`, `TestToggleVPNCallsAPI`, `TestRebootShowsConfirmAndCallsAPI`
- `system_handlers_test.go`: `TestRebootHandler` (add to existing)
- Update `use-vpn.ts` test (if exists) to verify enable state is passed

**Acceptance criteria:**
- "Restart WiFi" button triggers disconnect + reconnect API calls
- "Toggle VPN" button calls wireguard toggle with enable state
- "Reboot System" button shows confirmation then calls reboot endpoint
- Uptime displays real API data, not hardcoded 86432
- `formatBytes`/`formatUptime` exist only in `utils.ts`
- WiFi scan page loads with auto-scan enabled

**Estimated complexity:** M

---

## Phase 5: Real Data for Network Clients, Storage, CPU + JWT Blocklist + Rate Limiting

**Objective:** Replace all remaining hardcoded/stubbed data in backend services with real implementations, add JWT blocklist for logout, and add login rate-limiting.

**Issues addressed:** #12 (JWT blocklist for logout), #13 (Login rate-limiting), #15 (Hardcoded clients), #16 (Hardcoded storage), #17 (CPU = load average)

**Files to modify:**
- `backend/internal/services/network_service.go` — `GetNetworkStatus()`: Replace hardcoded clients array with DHCP lease query via `ubus call dhcp ipv4leases` or parsing `/tmp/dhcp.leases`
- `backend/internal/services/system_service.go` — `GetSystemStats()`:
  - Storage: read from `statvfs` syscall or parse `df /` output
  - CPU: read `/proc/stat` deltas between two samples (or at least normalize load average per core count)
- `backend/internal/auth/auth.go` — Add `blocklist map[string]time.Time` to `AuthService`; `Logout(tokenStr)` adds token to blocklist; `ValidateToken()` checks blocklist; add periodic cleanup of expired entries
- `backend/internal/auth/auth.go` — Add `rateLimiter` using a token bucket or sliding window (per-IP, e.g. 5 attempts per minute); apply in `Login()` or as middleware on `/auth/login`
- `backend/internal/api/auth_handlers.go` — `LogoutHandler` extracts token and calls `auth.Logout()`
- `backend/internal/api/router.go` — Ensure logout route calls updated handler

**Tests to write (TDD):**
- `network_service_test.go`: `TestGetClientsFromDHCPLeases` — mock ubus returns lease data, verify parsed into `Client` structs
- `system_service_test.go`: `TestGetStorageFromFS`, `TestGetCPUUsageFromProcStat`
- `auth_test.go`: `TestLogoutBlocklistsToken`, `TestBlocklistedTokenRejected`, `TestExpiredBlocklistEntryCleanedUp`
- `auth_test.go`: `TestLoginRateLimitExceeded` — 6th attempt within 60s returns 429

**Acceptance criteria:**
- Network clients are queried from DHCP leases (mock returns realistic data in mock mode)
- Storage stats come from filesystem (mock mode still returns mock data)
- CPU usage is calculated from `/proc/stat` deltas or properly normalized load
- Logout invalidates the JWT token; subsequent requests with that token return 401
- 6th login attempt within 60s from same IP returns 429 Too Many Requests

**Estimated complexity:** L

---

## Phase 6: Mobile Sidebar + WireGuard .conf Parser + CORS

**Objective:** Add responsive mobile sidebar with hamburger menu, implement WireGuard `.conf` file parser for import, and add configurable CORS middleware.

**Issues addressed:** #11 (Mobile sidebar), #25 (WireGuard .conf parser), #26 (CORS middleware)

**Files to modify:**
- `frontend/src/components/layout/app-shell.tsx` — Add hamburger button visible on `md:hidden`, slide-out drawer overlay for navigation on mobile, close on route change
- `frontend/src/components/layout/sidebar.tsx` — Extract sidebar content into reusable component shared between desktop sidebar and mobile drawer
- `backend/internal/services/vpn_service.go` — Add `ParseWireguardConf(confText string) (WireguardConfig, error)` that parses `[Interface]` and `[Peer]` sections
- `backend/internal/api/vpn_handlers.go` — Add `ImportWireguardHandler` for `POST /api/v1/vpn/wireguard/import` accepting `{ config: "<.conf text>" }`
- `backend/internal/api/router.go` — Register import route
- `backend/cmd/server/main.go` — Add CORS middleware with configurable allowed origins (env var `CORS_ORIGINS`, default `*` in dev)
- `backend/internal/config/config.go` — Add `CORSOrigins` field

**Files to create:**
- `frontend/src/components/layout/__tests__/mobile-sidebar.test.tsx` — Test hamburger shows on mobile, drawer opens/closes, navigation works
- `backend/internal/services/wireguard_parser.go` — Standalone parser module
- `backend/internal/services/wireguard_parser_test.go` — Comprehensive parser tests

**Tests to write (TDD):**
- `wireguard_parser_test.go`: `TestParseBasicConf`, `TestParseMultiplePeers`, `TestParseMissingInterface`, `TestParseEmptyConf`, `TestParseWithComments`
- `mobile-sidebar.test.tsx`: `TestHamburgerVisibleOnMobile`, `TestDrawerOpensOnClick`, `TestDrawerClosesOnRouteChange`
- Verify CORS headers present in responses with `TestCORSHeaders`

**Acceptance criteria:**
- On screens < 768px, sidebar is hidden and hamburger button appears
- Tapping hamburger opens slide-out navigation drawer
- WireGuard `.conf` file can be pasted/uploaded and parsed into config fields
- CORS headers are set according to `CORS_ORIGINS` env var
- All tests pass

**Estimated complexity:** M

---

## Phase 7: Error Boundaries + Toast Notifications + Form Validation Feedback

**Objective:** Add React error boundaries to prevent full-app crashes, add a toast/notification system for mutation feedback, and add field-level validation to forms.

**Issues addressed:** #21 (No error boundaries), #22 (No toast/notification), #28 (No form validation feedback), #29 (Login page basic)

**Files to create:**
- `frontend/src/components/ui/error-boundary.tsx` — Generic `ErrorBoundary` component with fallback UI and retry button
- `frontend/src/components/ui/toaster.tsx` — Toast notification system (using Sonner or custom implementation)
- `frontend/src/lib/validation.ts` — Shared validation functions: `validateIP`, `validateSSID`, `validateMTU`, `validateWireguardKey`

**Files to modify:**
- `frontend/src/App.tsx` — Wrap app in `<ErrorBoundary>`
- `frontend/src/router.tsx` — Add per-route error boundaries
- `frontend/src/hooks/use-wifi.ts` — Add `onSuccess`/`onError` toast notifications to mutations
- `frontend/src/hooks/use-network.ts` — Add toast notifications to mutations
- `frontend/src/hooks/use-vpn.ts` — Add toast notifications to mutations
- `frontend/src/hooks/use-services.ts` — Add toast notifications to mutations
- `frontend/src/pages/wifi/wifi-page.tsx` — Add field-level validation for SSID (required), password (min length based on encryption)
- `frontend/src/pages/network/network-page.tsx` — Add field-level validation for IP, netmask, gateway, MTU
- `frontend/src/pages/vpn/vpn-page.tsx` — Add field-level validation for WireGuard fields
- `frontend/src/pages/login/login-page.tsx` — Add branding/router image placeholder, improve styling, add "remember me" checkbox (persists token in localStorage vs sessionStorage)

**Tests to write (TDD):**
- `error-boundary.test.tsx`: `TestCatchesRenderError`, `TestShowsFallbackUI`, `TestRetryButton`
- `validation.test.ts`: `TestValidateIP`, `TestValidateSSID`, `TestValidateMTU`, `TestValidateWireguardKey`
- `login-page.test.tsx`: Update to test new layout, "remember me" toggle

**Acceptance criteria:**
- A component throw does not crash the entire app; error boundary catches and shows fallback
- Mutations display success/error toasts
- WiFi connect form shows field-level errors (empty SSID, short password)
- WAN config form shows field-level errors (invalid IP, MTU out of range)
- Login page has improved styling with branding area and "remember me"
- All tests pass

**Estimated complexity:** M

---

## Phase 8: Polish — Animations, PWA, Bandwidth Graphs, Shadcn/UI Upgrade

**Objective:** Final polish pass — page transitions, PWA manifest + favicon, bandwidth/usage graphs on dashboard, and evaluate Shadcn/UI component upgrade for accessibility.

**Issues addressed:** #23 (Shadcn/UI accessibility), #24 (Bandwidth/usage graphs), #27 (Favicon + PWA manifest), #30 (No animations/transitions)

**Files to create:**
- `frontend/public/manifest.json` — PWA manifest with app name, icons, theme color
- `frontend/public/favicon.svg` — SVG favicon (router icon)
- `frontend/src/pages/dashboard/bandwidth-chart.tsx` — Recharts area chart for bandwidth usage over time (WS-fed data)
- `frontend/src/components/ui/page-transition.tsx` — Framer Motion or CSS-based page transition wrapper

**Files to modify:**
- `frontend/index.html` — Add manifest link, favicon link, theme-color meta
- `frontend/src/router.tsx` — Wrap route outlets in page transition component
- `frontend/src/pages/dashboard/dashboard-page.tsx` — Add `BandwidthChart` component
- `frontend/src/components/ui/*.tsx` — Audit existing hand-rolled components; replace with Shadcn/UI (Radix-based) where missing: Dialog, DropdownMenu, Sheet (for mobile drawer), Toast
- `backend/internal/ws/hub.go` — Add bandwidth data to WS broadcast payload (tx_bytes, rx_bytes from `/proc/net/dev` or ubus)

**Tests to write (TDD):**
- `bandwidth-chart.test.tsx`: `TestRendersChart`, `TestUpdatesWithWSData`
- `page-transition.test.tsx`: `TestTransitionWrapsChildren`
- Verify PWA manifest is valid JSON with required fields

**Acceptance criteria:**
- App is installable as PWA (manifest + service worker registration)
- Favicon appears in browser tab
- Bandwidth chart renders on dashboard with live WS data
- Page transitions animate smoothly (fade or slide)
- Shadcn/UI Dialog, Sheet, Toast, DropdownMenu are used where applicable
- Lighthouse accessibility score ≥ 90
- All tests pass

**Estimated complexity:** L

---

## Summary

| Phase | Title                                       | Issues                  | Complexity | Dependencies                   |
| ----- | ------------------------------------------- | ----------------------- | ---------- | ------------------------------ |
| 1     | Real UCI/ubus + Graceful Shutdown           | #1, #18                 | L          | None                           |
| 2     | Captive Portal + WebSocket Auth             | #2, #3                  | M          | None                           |
| 3     | Shared Routes + API Client + Validation     | #5, #6, #7, #8, #14     | M          | None                           |
| 4     | Dashboard Quick Actions + Data Fixes        | #4, #9, #10, #19, #20   | M          | Phase 3 (uses API_ROUTES)      |
| 5     | Real Data + JWT Blocklist + Rate Limiting   | #12, #13, #15, #16, #17 | L          | Phase 1 (uses real UCI/ubus)   |
| 6     | Mobile Sidebar + WG Parser + CORS           | #11, #25, #26           | M          | None                           |
| 7     | Error Boundaries + Toasts + Form Validation | #21, #22, #28, #29      | M          | Phase 3 (validation functions) |
| 8     | Polish: Animations, PWA, Graphs, Shadcn/UI  | #23, #24, #27, #30      | L          | Phase 2 (WS), Phase 6 (mobile) |

**Recommended execution order:** Phases 1–3 can be done in parallel (no cross-dependencies). Phase 4 depends on 3. Phase 5 depends on 1. Phases 6–7 can run in parallel after 3. Phase 8 is last.

**Total: 30 issues, 8 phases, estimated ~3–4 weeks of development effort.**
