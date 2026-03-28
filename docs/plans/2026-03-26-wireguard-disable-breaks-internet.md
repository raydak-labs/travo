# WireGuard disable breaks internet — Fix Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Disabling WireGuard must restore the previous upstream default route so internet continues to work (no “stuck offline after disabling VPN”).

**Architecture:** Fix backend’s WireGuard toggle API contract (request body field name) and the disable-path recovery logic so it brings the *active* upstream interface back up (WWAN in travel-router mode) and reloads network, restoring the kernel default route.

**Tech Stack:** Go (Fiber), OpenWrt netifd/ubus, UCI, nftables (fw4), shell-based integration tests.

---

## Problem Statement (repro)

On a freshly booted router with VPN disabled:

- internet works
- enabling WireGuard works
- **disabling WireGuard makes internet stop working** (router cannot reach external endpoints)

Observed on the real device (`OpenWrt 25.12.1`) using the backend WireGuard toggle endpoint.

## Evidence collected from device

Artifacts were captured under `tmp/` from real device `192.168.1.1`.

### Key finding: after disabling WG, the kernel has **no default route**

After disable, `ifstatus wwan` still reports a default route exists (via DHCP), but:

- `ip -4 route show table main` contains **no** `default ...`
- `ip -4 route show default` outputs nothing
- router connectivity check fails (`wget_exit_code=4`)

This directly explains “curl/wget to the internet stops working”.

### Key finding: API request body mismatch can silently disable instead of enable

Backend handler expected `{"enable": true|false}` while OpenAPI + frontend conventions use `{"enabled": ...}`.

Result: a client sending `{"enabled": true}` could be parsed as “enable=false” and would **disable** WireGuard while returning `{"status":"ok"}`.

## Root cause hypothesis

1. **Disable path recovery targets the wrong interface.**
   - `VpnService.ToggleWireguard(false)` brings up `network.interface.wan` / `wan6` only.
   - In travel-router mode the upstream is typically `wwan` (WiFi STA).
   - So routes don’t get rebuilt for the actually-active uplink, leaving the kernel without a default route.

2. **API contract mismatch (`enabled` vs `enable`) hides real behavior.**
   - Clients can “think” they enabled while the backend actually disabled (or did nothing), creating confusing state transitions.

## Fix design

### A) Accept `enabled` (and keep `enable` for backward-compat)

- Update `ToggleWireguardHandler` and `ToggleTailscaleHandler` to:
  - prefer `enabled` when present
  - fall back to `enable` for older clients

### B) Restore routing on disable for WWAN mode

On disabling WireGuard:

- bring down `wg0`
- bring up **both** `wan/wan6` and `wwan/wwan6` (best-effort)
- reload `network`
- validate the kernel has a default route again (integration check)

This is deliberately conservative and avoids having to perfectly detect the current uplink source.

## Implementation tasks

### Task 1: Add failing unit tests for toggle request parsing

**Files:**
- Modify: `backend/internal/api/vpn_handlers.go`
- Modify: `backend/internal/api/vpn_handlers_test.go`

- [ ] **Step 1: Write failing tests**
  - POST `/api/v1/vpn/wireguard/toggle` with `{"enabled":true}` should enable WG (reflected in `/api/v1/vpn/status`).
  - POST `/api/v1/vpn/wireguard/toggle` with `{"enable":true}` should also work (backward-compat).
- [ ] **Step 2: Run tests to verify red**
  - Run: `cd backend && go test ./internal/api -run ToggleWireguard`
- [ ] **Step 3: Implement handler parsing**
- [ ] **Step 4: Run tests to verify green**

### Task 2: Fix disable-path recovery in `VpnService.ToggleWireguard(false)`

**Files:**
- Modify: `backend/internal/services/vpn_service.go`

- [ ] **Step 1: Add (or extend) a unit test** in `backend/internal/services/vpn_service_test.go`
  - Use a mock command runner that records calls.
  - Assert that disable path calls:
    - `ifdown wg0`
    - `ubus call network.interface.wwan up` (in addition to `wan`).
- [ ] **Step 2: Implement change**
- [ ] **Step 3: Run service tests**
  - Run: `cd backend && go test ./internal/services -run ToggleWireguard`

### Task 3: Add real-device integration test (bash) for regression

**Files:**
- Create: `scripts/integration-vpn-wireguard-toggle.sh`

- [ ] **Step 1: Add script**
  - Flow:
    - login
    - ensure WG off
    - baseline connectivity check from router
    - enable WG -> connectivity ok
    - disable WG -> connectivity **must remain ok**
  - Save artifacts under `tmp/integration-vpn-wireguard-toggle-<ts>/`
- [ ] **Step 2: Validate it fails on current device build**
  - Run: `./scripts/integration-vpn-wireguard-toggle.sh`
  - Expected: FAIL before deployment of fix
- [ ] **Step 3: Deploy and re-run**
  - After deploying backend build to device, rerun and expect PASS.

### Task 4: Deploy + verify on real device

- [ ] Deploy backend to router (using existing project deploy process)
- [ ] Run `./scripts/integration-vpn-wireguard-toggle.sh`
- [ ] Manually verify:
  - `ip route show default` exists after disabling WG
  - `wget`/`curl` to external works after disabling WG

## Rollout / compatibility notes

- Accepting both `enabled` and `enable` avoids breaking older clients.
- The disable recovery bringing up both WAN and WWAN is safe as “best-effort”; if an interface doesn’t exist, ubus returns error which is ignored.

