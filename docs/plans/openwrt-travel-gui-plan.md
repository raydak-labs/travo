---
title: "Plan"
description: "Planning / design notes: Plan"
updated: 2026-04-13
tags: [openwrt, plan, traceability]
---

## Plan: OpenWRT Travel GUI

A modern, mobile-first web UI for OpenWRT-based travel routers (GL.iNet Beryl AX MT3000, Slate AXT1800), providing an intuitive dashboard, WiFi management with hotel captive portal support, VPN/service management, and system configuration. Built as a Go backend + React frontend monorepo that coexists with LuCI (taking port 80/443, relocating LuCI to an alternate port). Licensed under MIT.

### Decisions

- **Name:** openwrt-travel-gui (for now)
- **License:** MIT
- **Authentication:** Yes — session-based login with configurable password
- **Target Devices:** GL.iNet Beryl AX (MT3000), Slate AXT1800 (both aarch64)
- **LuCI Coexistence:** This UI claims port 80/443; LuCI is moved to port 8080 via uhttpd config

### Architecture

```
┌─────────────────────────────────────────────┐
│              Browser (React SPA)            │
│  TanStack Query/Router, Shadcn/UI, Zustand  │
└──────────────────┬──────────────────────────┘
                   │ REST + WebSocket
┌──────────────────▼──────────────────────────┐
│           Go Backend (Fiber)                │
│  ┌─────────┐ ┌──────────┐ ┌──────────────┐ │
│  │ Auth    │ │ REST API │ │ WebSocket    │ │
│  │ (session│ │ handlers │ │ (live stats) │ │
│  │  +JWT)  │ │          │ │              │ │
│  └────┬────┘ └────┬─────┘ └──────┬───────┘ │
│       │           │               │         │
│  ┌────▼───────────▼───────────────▼───────┐ │
│  │        Service Layer                   │ │
│  │  UCI wrapper │ ubus client │ opkg mgr  │ │
│  └────────────────────────────────────────┘ │
│       │ (mock mode: in-memory fakes)        │
└───────┼─────────────────────────────────────┘
        │ exec / socket
┌───────▼─────────────────────────────────────┐
│            OpenWRT System                   │
│  UCI configs │ ubus │ init.d │ iwinfo      │
└─────────────────────────────────────────────┘
```

---

**Phases (7 phases)**

### 1. Phase 1: Monorepo Scaffolding & Tooling

- **Objective:** Set up the pnpm monorepo with frontend, backend, and shared workspace packages. Configure linting, formatting, TypeScript, Go module, and project documentation.
- **Files/Functions to Create:**
  - Root: `package.json`, `pnpm-workspace.yaml`, `.gitignore`, `.npmrc`, `tsconfig.base.json`, `.eslintrc.cjs`, `.prettierrc`, `Makefile`, `README.md`, `LICENSE`, `AGENTS.md`, `CONTRIBUTING.md`
  - `frontend/package.json`, `frontend/tsconfig.json`, `frontend/vite.config.ts` (scaffolded)
  - `backend/go.mod`, `backend/go.sum`, `backend/cmd/server/main.go` (hello-world)
  - `shared/package.json`, `shared/tsconfig.json`, `shared/src/index.ts`
  - `scripts/dev.sh` (concurrent frontend + backend dev)
- **Tests to Write:**
  - `backend/cmd/server/main_test.go` — server starts and responds on health endpoint
  - `shared/src/__tests__/index.test.ts` — shared package exports correctly
- **Steps:**
  1. Initialize git repo, create `.gitignore` for Node, Go, and IDE files
  2. Create pnpm workspace with `frontend/`, `backend/`, `shared/` packages
  3. Configure root TypeScript, ESLint (flat config), Prettier
  4. Initialize Go module (`backend/`) with Fiber dependency, write `main.go` with `/api/health` endpoint
  5. Write test for health endpoint, run it, verify it passes
  6. Write shared package with placeholder type export and test
  7. Create `Makefile` with targets: `dev`, `build`, `test`, `lint`, `clean`, `format`
  8. Write `README.md` with project vision, architecture diagram, quickstart, and contributing guide
  9. Create `LICENSE` (MIT) and `CONTRIBUTING.md`

### 2. Phase 2: Shared API Contract & Types

- **Objective:** Define TypeScript interfaces and Go structs for the full API surface — system info, network, WiFi, VPN, services, clients, auth. This becomes the single source of truth for the API contract.
- **Files/Functions to Create:**
  - `shared/src/api/system.ts` — `SystemInfo`, `CpuStats`, `MemoryStats`, `StorageStats`, `Uptime`
  - `shared/src/api/network.ts` — `NetworkStatus`, `WanConfig`, `LanConfig`, `DhcpLease`, `Client`
  - `shared/src/api/wifi.ts` — `WifiNetwork`, `WifiScanResult`, `WifiConnection`, `WifiMode`
  - `shared/src/api/vpn.ts` — `WireguardConfig`, `WireguardPeer`, `VpnStatus`
  - `shared/src/api/services.ts` — `ServiceInfo`, `ServiceStatus`, `TailscaleStatus`, `AdGuardStatus`
  - `shared/src/api/auth.ts` — `LoginRequest`, `LoginResponse`, `Session`
  - `shared/src/api/captive.ts` — `CaptivePortalStatus`, `DnsOverride`
  - `shared/src/api/routes.ts` — API route constants
  - `backend/internal/models/` — corresponding Go structs
- **Tests to Write:**
  - `shared/src/api/__tests__/types.test.ts` — type guard validation tests for each model
  - `backend/internal/models/models_test.go` — JSON marshal/unmarshal roundtrip tests
- **Steps:**
  1. Write TypeScript interfaces for all API domains (system, network, wifi, vpn, services, auth, captive portal)
  2. Write type guard functions and validation helpers
  3. Write tests for type guards, run them, verify they fail, then implement
  4. Define API route constants as shared module
  5. Write corresponding Go structs in `backend/internal/models/` with JSON tags
  6. Write Go roundtrip tests, run them, verify they pass
  7. Export everything from shared package entry point

### 3. Phase 3: Go Backend Core + Mock Mode

- **Objective:** Build the Go backend with Fiber, session-based auth, REST API handlers, WebSocket for live stats, UCI/ubus abstraction layer, and `--mock` flag for local development without a router.
- **Files/Functions to Create:**
  - `backend/cmd/server/main.go` — CLI flags, server bootstrap, graceful shutdown
  - `backend/internal/config/config.go` — server config, CLI flag parsing
  - `backend/internal/auth/` — session store, login handler, middleware
  - `backend/internal/api/` — route registration, handlers for each domain
  - `backend/internal/uci/` — `UCI` interface + `RealUCI` and `MockUCI` implementations
  - `backend/internal/ubus/` — `Ubus` interface + `RealUbus` and `MockUbus` implementations
  - `backend/internal/services/` — service layer (system, network, wifi, vpn, services)
  - `backend/internal/ws/` — WebSocket hub for live stats broadcasting
  - `backend/internal/mock/` — realistic mock data generators
- **Tests to Write:**
  - `backend/internal/auth/auth_test.go` — login, session validation, middleware rejection
  - `backend/internal/api/system_test.go` — system info handler returns valid data
  - `backend/internal/api/network_test.go` — network status, WAN config CRUD
  - `backend/internal/api/wifi_test.go` — scan, connect, disconnect, mode switch
  - `backend/internal/uci/mock_test.go` — mock UCI get/set/commit operations
  - `backend/internal/ubus/mock_test.go` — mock ubus call responses
- **Steps:**
  1. Write tests for auth handlers (login success, login failure, session expiry, middleware blocks unauthenticated)
  2. Implement auth package: session store, bcrypt password check, Fiber middleware
  3. Write UCI interface and MockUCI implementation with tests
  4. Write ubus interface and MockUbus implementation with tests
  5. Write tests for system info API handler, then implement handler + service layer
  6. Write tests for network status API handler, then implement
  7. Write tests for WiFi scan/connect API handlers, then implement
  8. Implement WebSocket hub for live system stats (CPU, memory, connections, bandwidth)
  9. Add `--mock` CLI flag that injects mock implementations of UCI/ubus
  10. Wire all routes in main.go, verify all tests pass

### 4. Phase 4: Frontend Shell & Dashboard

- **Objective:** Scaffold the React app with Vite, configure TailwindCSS + Shadcn/UI, build the app shell (responsive sidebar, header, theme toggle), set up MSW for API mocking, and build the main Dashboard page with live data.
- **Files/Functions to Create:**
  - `frontend/src/main.tsx`, `frontend/src/App.tsx`
  - `frontend/src/lib/api-client.ts` — typed fetch wrapper using shared types
  - `frontend/src/lib/ws-client.ts` — WebSocket client for live stats
  - `frontend/src/components/layout/` — `AppShell`, `Sidebar`, `Header`, `ThemeProvider`
  - `frontend/src/components/ui/` — Shadcn/UI components (installed via CLI)
  - `frontend/src/pages/dashboard/` — `DashboardPage`, `ConnectionStatusCard`, `VpnStatusCard`, `QuickActionsBar`, `SystemStatsCard`, `ConnectedClientsCard`
  - `frontend/src/pages/login/` — `LoginPage`, `LoginForm`
  - `frontend/src/mocks/` — MSW handlers and mock data
  - `frontend/src/routes.tsx` — TanStack Router file-based routes
- **Tests to Write:**
  - `frontend/src/pages/login/__tests__/LoginPage.test.tsx` — renders form, submits credentials, handles error
  - `frontend/src/pages/dashboard/__tests__/DashboardPage.test.tsx` — renders all cards, shows connection status, quick actions work
  - `frontend/src/components/layout/__tests__/AppShell.test.tsx` — responsive sidebar, theme toggle
  - `frontend/src/lib/__tests__/api-client.test.ts` — request formatting, error handling, auth redirect
- **Steps:**
  1. Scaffold Vite + React + TypeScript project
  2. Install and configure TailwindCSS 4 + Shadcn/UI + dark/light theme (system preference detection)
  3. Set up MSW with handlers that return shared mock data for all API endpoints
  4. Write tests for LoginPage (form render, submit, error states), implement LoginPage
  5. Write tests for AppShell (sidebar navigation, responsive collapse, theme toggle), implement layout
  6. Write tests for DashboardPage, then implement:
     - Connection status card (WiFi name/signal or Ethernet or Mobile with icon)
     - VPN status card (WireGuard/Tailscale active indicator + quick toggle)
     - Quick actions bar (switch WiFi, toggle VPN, reboot)
     - System stats card (CPU, RAM, uptime as mini gauges)
     - Connected clients count card
  7. Set up TanStack Router with routes: `/login`, `/dashboard`, `/wifi`, `/network`, `/vpn`, `/services`, `/system`
  8. Configure TanStack Query with auth-aware defaults (redirect to login on 401)
  9. Verify all tests pass, visual check of dashboard in browser

### 5. Phase 5: WiFi Management & Hotel Captive Portal

- **Objective:** Build the WiFi management pages (scan, connect, mode switching, saved networks) and captive portal detection with redirect-to-login functionality.
- **Files/Functions to Create:**
  - `backend/internal/api/wifi_handlers.go` — scan, connect, disconnect, mode set, saved networks CRUD
  - `backend/internal/api/captive_handlers.go` — captive portal status, DNS override
  - `backend/internal/services/wifi_service.go` — wraps iwinfo scan, wpa_supplicant, UCI wifi config
  - `backend/internal/services/captive_service.go` — HTTP probe to detectportal.firefox.com, DNS check
  - `frontend/src/pages/wifi/` — `WifiPage`, `WifiScanList`, `WifiConnectDialog`, `WifiModeSelector`, `SavedNetworks`
  - `frontend/src/components/wifi/` — `SignalStrengthIcon`, `SecurityBadge`, `CaptivePortalBanner`
- **Tests to Write:**
  - `backend/internal/services/wifi_service_test.go` — scan parsing, connect flow, mode switch
  - `backend/internal/services/captive_service_test.go` — portal detected, portal not detected, DNS override
  - `frontend/src/pages/wifi/__tests__/WifiPage.test.tsx` — scan results render, connect flow, mode selector
  - `frontend/src/components/wifi/__tests__/CaptivePortalBanner.test.tsx` — shows/hides based on status, opens login
- **Steps:**
  1. Write backend WiFi service tests (scan result parsing, connect sequence, mode switching between client/repeater/AP)
  2. Implement WiFi service wrapping iwinfo/wpa_supplicant/UCI (with mock fallback)
  3. Write captive portal detection tests (HTTP probe, DNS resolution check)
  4. Implement captive portal service: probe known URLs, detect redirects, provide portal URL
  5. Write WiFi API handler tests, implement handlers
  6. Write frontend WiFi page tests, then implement:
     - Scan results list with signal strength bars, security icons, frequency band badges
     - Connect dialog with password input, advanced options
     - Mode selector (Client / Repeater / Access Point) with visual explanation
     - Saved networks list with auto-connect toggle, priority ordering
  7. Implement CaptivePortalBanner: persistent notification when portal detected, "Login to Network" button opens portal URL in iframe/new tab
  8. Add captive portal status to dashboard connection card
  9. Verify all tests pass

### 6. Phase 6: VPN & Services Management

- **Objective:** Build the plugin/service management system for installing, configuring, and controlling WireGuard, Tailscale, AdGuard Home, and extensible for future services. Services show on dashboard when enabled.
- **Files/Functions to Create:**
  - `backend/internal/api/services_handlers.go` — list available, install, remove, configure, start, stop, status
  - `backend/internal/api/vpn_handlers.go` — WireGuard and Tailscale specific endpoints
  - `backend/internal/services/package_service.go` — wraps opkg for install/remove
  - `backend/internal/services/service_manager.go` — wraps init.d for start/stop/enable/disable/status
  - `backend/internal/services/wireguard_service.go` — WireGuard config import, peer management
  - `backend/internal/services/tailscale_service.go` — Tailscale auth, status, exit node
  - `backend/internal/services/adguard_service.go` — AdGuard Home install, enable, stats
  - `frontend/src/pages/services/` — `ServicesPage`, `ServiceCard`, `InstallDialog`
  - `frontend/src/pages/vpn/` — `VpnPage`, `WireguardConfig`, `TailscaleStatus`, `VpnToggle`
  - `frontend/src/pages/vpn/components/` — `WireguardImport`, `PeerList`, `TailscaleAuth`, `ExitNodeSelector`
- **Tests to Write:**
  - `backend/internal/services/package_service_test.go` — install, remove, list installed
  - `backend/internal/services/wireguard_service_test.go` — config import, peer add/remove, toggle
  - `backend/internal/services/tailscale_service_test.go` — auth URL, status, exit node set
  - `backend/internal/services/adguard_service_test.go` — install, enable, query stats
  - `frontend/src/pages/vpn/__tests__/VpnPage.test.tsx` — WireGuard config, Tailscale auth, toggles
  - `frontend/src/pages/services/__tests__/ServicesPage.test.tsx` — available list, install flow, status display
- **Steps:**
  1. Write package service tests (opkg list, install, remove), implement with mock
  2. Write service manager tests (init.d start/stop/enable/status), implement with mock
  3. Write WireGuard service tests (import .conf file, parse peers, toggle interface), implement
  4. Write Tailscale service tests (get auth URL, check status, set exit node), implement
  5. Write AdGuard Home service tests (install via opkg, configure, get filter stats), implement
  6. Write VPN API handler tests, implement handlers
  7. Write frontend VPN page tests, then implement:
     - WireGuard: import config textarea/file upload, peer list, status indicator, toggle
     - Tailscale: auth flow (show URL/QR), connection status, exit node dropdown, toggle
  8. Write frontend Services page tests, then implement:
     - Available services grid with install/remove buttons
     - Service status cards with start/stop/configure actions
     - AdGuard Home: enable toggle, query stats display
  9. Update Dashboard: dynamically show VPN/service cards when services are installed and enabled
  10. Verify all tests pass

### 7. Phase 7: Docker Dev Environment, Build Pipeline & Deployment

- **Objective:** Create Docker Compose setup for local development, cross-compilation scripts for aarch64 (Beryl AX, Slate AXT1800), `.ipk` packaging, LuCI port relocation, and deployment documentation.
- **Files/Functions to Create:**
  - `Dockerfile.dev` — Go backend dev container with hot reload
  - `docker-compose.yml` — frontend dev + backend mock + optional OpenWRT rootfs mock
  - `scripts/build.sh` — cross-compile Go for aarch64, bundle frontend assets
  - `scripts/package-ipk.sh` — create `.ipk` package with control files
  - `scripts/deploy.sh` — scp + install on target device
  - `packaging/openwrt/Makefile` — OpenWRT SDK package Makefile
  - `packaging/openwrt/files/` — init.d script, UCI defaults (port config, LuCI relocation)
  - `packaging/openwrt/files/etc/init.d/openwrt-travel-gui` — procd service definition
  - `packaging/openwrt/files/etc/uci-defaults/99-travel-gui-ports` — moves LuCI to 8080, binds travel-gui to 80/443
  - `docs/deployment.md` — full deployment guide
  - `docs/development.md` — local dev setup guide
- **Tests to Write:**
  - `scripts/test-build.sh` — verify cross-compiled binary runs (via QEMU if available)
  - `scripts/test-package.sh` — verify `.ipk` structure is valid
- **Steps:**
  1. Create `Dockerfile.dev` with Go + Air (hot reload) for backend development
  2. Create `docker-compose.yml`: frontend (Vite dev server), backend (mock mode with hot reload)
  3. Write `scripts/build.sh`: compile frontend (vite build), embed in Go binary, cross-compile for `linux/arm64` (aarch64 for MT3000 and AXT1800)
  4. Write `scripts/package-ipk.sh`: create proper `.ipk` with control file, conffiles, postinst/prerm scripts
  5. Write procd init.d service script for automatic startup
  6. Write UCI defaults script that: changes uhttpd (LuCI) listen port to 8080, configures travel-gui on 80/443
  7. Write `packaging/openwrt/Makefile` for OpenWRT SDK integration
  8. Write `scripts/deploy.sh`: scp binary to router, install, restart service
  9. Test build pipeline end-to-end, verify `.ipk` installs correctly
  10. Write `docs/deployment.md` covering: prerequisites, build from source, install via opkg, first-run setup, updating, uninstalling (restores LuCI to 80)
  11. Write `docs/development.md` covering: prerequisites, clone, pnpm install, docker compose up, running tests, architecture overview
