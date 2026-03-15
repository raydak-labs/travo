# Implementation Guide: Simple Tasks

> Items that are straightforward to implement without a full plan document.
> Each item includes the requirement reference, what needs to change, and which files to touch.

---

## Table of Contents

- [0.4 — Dark Mode Toggle Color](#04--dark-mode-toggle-color)
- [1.2 — AP Disable with Warnings](#12--ap-disable-with-warnings)
- [1.4 — Multi-Radio Per-Radio Config](#14--multi-radio-per-radio-config)
- [2.6 — Detect Ethernet Cable Plug-In](#26--detect-ethernet-cable-plug-in)
- [3.1 — Install luci-proto-wireguard](#31--install-luci-proto-wireguard)
- [3.3 — VPN Data Usage on Dashboard](#33--vpn-data-usage-on-dashboard)
- [3.3 — DNS Leak Test](#33--dns-leak-test)
- [4.3 — WireGuard Post-Install Wizard](#43--wireguard-post-install-wizard)
- [4.4 — Tailscale Post-Install Auth Flow](#44--tailscale-post-install-auth-flow)
- [5.2 — Mark Reboot as Working + Add Shutdown](#52--mark-reboot-as-working--add-shutdown)
- [5.3 — Auto-Set Timezone from Browser](#53--auto-set-timezone-from-browser)
- [5.3 — Time Sync Travel Validation](#53--time-sync-travel-validation)
- [6.2 — Connection Uptime Log](#62--connection-uptime-log)
- [7 — HTTPS / TLS Support](#7--https--tls-support)
- [12 — Tooltips for Technical Fields](#12--tooltips-for-technical-fields)
- [13 — Init Script for Auto-Start](#13--init-script-for-auto-start)
- [13 — Size Optimization Audit](#13--size-optimization-audit)

---

## 0.4 — Dark Mode Toggle Color

**Requirement:** Toggles should be bluish in dark mode
**Status:** ✅ Already implemented — verify and mark as done.

The switch component at `frontend/src/components/ui/switch.tsx` already has `dark:peer-checked:bg-blue-500`. This was fixed in the last bug round.

**Action:** Mark `[x]` in requirements.md.

---

## 1.2 — AP Disable with Warnings

**Requirement:** Add options to disable individual AP interfaces with clear warnings
**Scope:** Single AP disable only. STA connections can be deleted without warning. AP disable needs a confirmation warning.

**Implementation:**
1. AP enable/disable toggle already exists per radio in the WiFi page
2. Add a confirmation dialog when disabling an AP: _"Disabling this access point will disconnect all clients currently connected to it. Are you sure?"_
3. If disabling the **last** remaining AP, show a stronger warning: _"⚠️ This is the only active access point. Disabling it will make the router unreachable via WiFi. You will need a wired connection or the other radio's AP to access the router."_

**Files:**
- `frontend/src/pages/wifi/wifi-page.tsx` — add confirmation dialog to AP toggle
- No backend changes needed (enable/disable AP already works)

**Effort:** ~1 hour

---

## 1.4 — Multi-Radio Per-Radio Config

**Requirement:** Per-radio configuration (one for uplink, one for AP), recommended config based on hardware
**Scope:** UI to assign radio roles.

**Implementation:**
1. Detect radios and their capabilities (already done — `radio_info` in WiFi API)
2. Add a "Radio Assignment" card in WiFi settings:
   - Radio 0 (5 GHz): Role dropdown → AP / STA / Both (Repeater)
   - Radio 1 (2.4 GHz): Role dropdown → AP / STA / Both (Repeater)
3. "Recommended" badge on the default config (5 GHz AP, 2.4 GHz STA)
4. Backend: when role changes, create/remove appropriate wireless interfaces

**Files:**
- `frontend/src/pages/wifi/wifi-page.tsx` — radio assignment card
- `backend/internal/services/wifi_service.go` — `SetRadioRole()` method
- `backend/internal/api/wifi_handlers.go` — new endpoint

**Effort:** ~4 hours

---

## 2.6 — Detect Ethernet Cable Plug-In

**Requirement:** Detect ethernet cable plug-in and auto-switch to wired WAN
**Scope:** Show notification when cable is connected/disconnected.

**Implementation:**
1. Backend: periodically check WAN interface carrier state via `/sys/class/net/eth0/carrier`
2. When carrier changes from 0→1: push WebSocket alert "Ethernet cable connected"
3. When carrier changes from 1→0: push alert "Ethernet cable disconnected"
4. Auto-switch: metric-based routing already handles this (WAN ethernet has lower metric than WWAN wifi)

**Files:**
- `backend/internal/services/alert_service.go` — add carrier state monitoring
- `backend/internal/ws/` — alert push

**Effort:** ~2 hours

---

## 3.1 — Install luci-proto-wireguard

**Requirement:** Also install luci-proto-wireguard for wireguard
**Scope:** Add to WireGuard package install list.

**Implementation:**
1. In the service registry, add `luci-proto-wireguard` to WireGuard's package list
2. Backend installs all three packages when WireGuard is installed: `wireguard-tools`, `kmod-wireguard`, `luci-proto-wireguard`

**Files:**
- `backend/internal/services/service_registry.go` (or service definitions) — add package to list

**Effort:** ~15 minutes

---

## 3.3 — VPN Data Usage on Dashboard

**Requirement:** Show VPN data usage on dashboard
**Scope:** Display wg0 transfer stats from `wg show`.

**Implementation:**
1. `GetWireGuardStatus()` already parses `wg show wg0 dump` which includes `transfer_rx` and `transfer_tx`
2. Add a small "VPN Traffic" section to the VPN status card on the dashboard
3. Show RX/TX since last handshake (data from wg dump)

**Files:**
- `frontend/src/pages/dashboard/` — add VPN traffic to VPN card
- `frontend/src/hooks/use-vpn.ts` — use existing WireGuard status hook

**Effort:** ~1 hour

---

## 3.3 — DNS Leak Test

**Requirement:** DNS leak test (verify traffic routes through VPN correctly)
**Scope:** Simple check that DNS queries go through VPN.

**Implementation:**
1. Backend endpoint: `POST /api/v1/vpn/dns-leak-test`
2. Steps:
   - Resolve a known domain (e.g., `whoami.akamai.net` or `o-o.myaddr.l.google.com`) via `nslookup`
   - Compare resolved IP with VPN server's expected DNS
   - Also check public IP via `curl https://api.ipify.org`
3. Return: `{ "public_ip": "...", "dns_server": "...", "vpn_ip_match": true, "dns_leak": false }`
4. Frontend: "DNS Leak Test" button in VPN page, shows results

**Files:**
- `backend/internal/services/vpn_service.go` — `DNSLeakTest()`
- `backend/internal/api/vpn_handlers.go` — handler
- `frontend/src/pages/vpn/vpn-page.tsx` — button + result display

**Effort:** ~3 hours

---

## 4.3 — WireGuard Post-Install Wizard

**Requirement:** Post-install setup wizard for WireGuard
**Scope:** After installing WireGuard package, guide user through importing a profile.

**Implementation:**
1. After successful WireGuard install from Services page, show a dialog: "WireGuard installed! Would you like to import a VPN configuration?"
2. Options: "Import .conf file" or "Configure manually" → navigate to VPN page
3. This is a frontend-only UX improvement

**Files:**
- `frontend/src/pages/services/services-page.tsx` — post-install dialog

**Effort:** ~1 hour

---

## 4.4 — Tailscale Post-Install Auth Flow

**Requirement:** Post-install authentication flow for Tailscale
**Scope:** After installing Tailscale, show auth URL.

**Implementation:**
1. After Tailscale install, backend starts `tailscaled` and runs `tailscale up`
2. Capture the auth URL from stdout
3. Frontend shows: "Open this link to authenticate your router: [auth URL]"
4. Poll `tailscale status` until authenticated
5. See [Tailscale Integration plan](tailscale-integration.md) for full details — this is Phase 1 of that plan.

**Files:**
- `backend/internal/services/vpn_service.go` — `StartTailscaleAuth()`
- `frontend/src/pages/services/services-page.tsx` — auth dialog

**Effort:** ~3 hours (basic flow)

---

## 5.2 — Mark Reboot as Working + Add Shutdown

**Requirement:** Reboot works (mark as done). Add a shutdown button.

**Implementation for shutdown:**
1. Backend: `POST /api/v1/system/shutdown` endpoint
2. Service: `func (s *SystemService) Shutdown() error` — runs `poweroff` command
3. Frontend: Add "Shutdown" button next to "Reboot" in system page with confirmation dialog: _"The device will power off. You will need physical access to turn it back on."_
4. Add "Shutdown" option to the actions dropdown in the header toolbar

**Files:**
- `backend/internal/services/system_service.go` — `Shutdown()` method
- `backend/internal/api/system_handlers.go` — `SystemShutdownHandler`
- `backend/internal/api/router.go` — register route
- `frontend/src/pages/system/system-page.tsx` — shutdown button + dialog
- `frontend/src/components/layout/` — actions dropdown

**Effort:** ~2 hours

---

## 5.3 — Auto-Set Timezone from Browser

**Requirement:** Clicking the timezone mismatch alert button should automatically set the timezone, not just navigate to the system page.

**Implementation:**
1. Modify `TimezoneAlert` component: the "Update" button should:
   - Detect browser timezone via `Intl.DateTimeFormat().resolvedOptions().timeZone`
   - Look up matching POSIX timezone string from the TIMEZONES constant
   - Call `PUT /api/v1/system/timezone` with the matched values
   - Show toast "Timezone updated to {zonename}"
   - Dismiss the alert
2. Fallback: if browser timezone doesn't match any known entry, navigate to system page (current behavior)

**Files:**
- `frontend/src/components/timezone-alert.tsx` — auto-set on click
- `frontend/src/hooks/use-system.ts` — use `useSetTimezone` mutation
- `frontend/src/pages/system/system-page.tsx` — TIMEZONES array should be extracted to a shared constant

**Effort:** ~1 hour

---

## 5.3 — Time Sync Travel Validation

**Requirement:** Validate time sync works correctly when traveling to other countries.

**Analysis:** The current implementation is correct for travel scenarios:
- NTP keeps the device clock correct in UTC
- Timezone display is a separate setting (zonename + POSIX timezone string)
- When traveling, the browser timezone changes → mismatch alert fires → user clicks "Update" → timezone is set
- Time sync on login (`/api/v1/system/time-sync`) handles devices without NTP (sets clock from browser)

**What to add:**
1. The auto-set timezone feature (above) makes this seamless
2. Optionally: after setting timezone, also call `date -s` to adjust if NTP hasn't synced yet
3. Add "Sync with NTP" button in Time & Timezone section that runs `ntpd -q -p pool.ntp.org`

**Files:**
- `backend/internal/services/system_service.go` — `SyncNTP()` method
- `backend/internal/api/system_handlers.go` — handler
- `frontend/src/pages/system/system-page.tsx` — "Sync NTP" button

**Effort:** ~2 hours

---

## 6.2 — Connection Uptime Log

**Requirement:** Connection uptime log (internet available since / lost at — timeline of events)
**Scope:** Track connectivity state changes, show as timeline.

**Implementation:**
1. Backend: periodic ping check (already done for captive portal detection)
2. Store state transitions in memory: `{ timestamp, state: "connected"|"disconnected", source: "wan"|"wwan" }`
3. Keep last 100 events in a ring buffer
4. API: `GET /api/v1/network/uptime-log`
5. Frontend: Timeline visualization in Network page — green/red bars showing connectivity periods

**Files:**
- `backend/internal/services/network_service.go` — connectivity state tracking
- `backend/internal/api/network_handlers.go` — handler
- `frontend/src/pages/network/uptime-log.tsx` — timeline component

**Effort:** ~4 hours

---

## 7 — HTTPS / TLS Support

**Requirement:** HTTPS / TLS support
**Priority:** Low

**Implementation (self-signed):**
1. On first run, generate a self-signed certificate: `openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -days 3650 -nodes -keyout /etc/openwrt-travel-gui/tls.key -out /etc/openwrt-travel-gui/tls.crt -subj "/CN=openwrt-travel-gui"`
2. Fiber supports TLS: `app.ListenTLS(":443", certFile, keyFile)`
3. Redirect HTTP→HTTPS (or run both)
4. Browser will show certificate warning (expected for self-signed)

**Alternative:** Use `uhttpd` (already on the device) as TLS reverse proxy → simpler, reuses existing OpenWRT TLS infrastructure.

**Files:**
- `backend/cmd/server/main.go` — TLS listener option
- `backend/internal/config/config.go` — TLS config flags
- `packaging/openwrt/files/etc/init.d/openwrt-travel-gui` — pass TLS flags

**Effort:** ~3 hours

---

## 12 — Tooltips for Technical Fields

**Requirement:** Tooltips for technical fields (what is MTU? what is DHCP range?)
**Scope:** Add info icon + hover tooltip next to technical input fields.

**Implementation:**
1. Create a reusable `InfoTooltip` component: info icon (ℹ️) that shows tooltip on hover
2. Add tooltips to:
   - MTU: "Maximum Transmission Unit — packet size limit. Default 1500. Lower for PPPoE (1492) or VPN tunnels (1420)."
   - DHCP range: "Range of IP addresses assigned to clients. E.g., 100-150 means 192.168.1.100 to 192.168.1.150"
   - Lease time: "How long a DHCP lease is valid before renewal. Default 12 hours."
   - DNS servers: "Domain Name System servers that resolve domain names to IP addresses."
   - Netmask: "Defines the network size. 255.255.255.0 (/24) supports 254 devices."
   - Gateway: "The router IP that forwards traffic to the internet."
   - SSID: "The name of the WiFi network visible to devices."
   - WPA key: "WiFi password. Must be at least 8 characters."

**Files:**
- `frontend/src/components/ui/info-tooltip.tsx` — reusable component
- `frontend/src/pages/network/network-page.tsx` — add to network fields
- `frontend/src/pages/wifi/wifi-page.tsx` — add to WiFi fields

**Effort:** ~2 hours

---

## 13 — Init Script for Auto-Start

**Requirement:** Init script for auto-start on boot
**Status:** ✅ Already implemented.

The init script at `packaging/openwrt/files/etc/init.d/openwrt-travel-gui` already exists as a procd service with `START=99`, respawn enabled, and auto-start. Verified on device: `/etc/init.d/openwrt-travel-gui enabled` returns true.

**Action:** Mark `[x]` in requirements.md.

---

## 13 — Size Optimization Audit

**Requirement:** Bundle size and Go binary optimization
**Scope:** Audit and reduce artifact sizes.

**Implementation:**
1. **Go binary:** Already uses `CGO_ENABLED=0`. Add `-ldflags="-s -w"` to strip debug info (~30% reduction). Current size: ~14MB → expected: ~10MB
2. **Frontend bundle:** Currently 1,091KB JS. Options:
   - Enable code-splitting per route (Vite lazy imports)
   - Tree-shake unused Lucide icons (import individually, not from barrel)
   - Check for duplicate dependencies in bundle analyzer
3. **Audit command:** Add `make size-audit` that reports:
   - Go binary size (stripped vs unstripped)
   - Frontend bundle size (gzipped)
   - Total deployment size

**Files:**
- `Makefile` — add `-ldflags="-s -w"` to build target, add `size-audit` target
- `frontend/vite.config.ts` — enable route-based code splitting
- `frontend/src/router.tsx` — lazy imports for route components

**Effort:** ~3 hours
