# OpenWRT Travel Router GUI — Feature Requirements

> **Last updated:** 2026-03-25 (v33 — mark all implemented features complete: scheduled WiFi, band switching, DNS-over-HTTPS, split tunneling, Tailscale SSH, NTP sync, SSH keys, alert thresholds, diagnostics, speed test, MAC policy, firewall zones, port forwarding, IPv6, WoL, PWA offline indicator)

---

## Table of Contents

- [1. WiFi Management](#1-wifi-management)
- [2. Network Management](#2-network-management)
- [3. VPN Management](#3-vpn-management)
- [4. Services Management](#4-services-management)
- [5. System Management](#5-system-management)
- [6. Dashboard & Monitoring](#6-dashboard--monitoring)
- [7. Authentication & Security](#7-authentication--security)
- [8. Captive Portal](#8-captive-portal)
- [9. Logs & Diagnostics](#9-logs--diagnostics)
- [10. Privacy & Identity](#10-privacy--identity)
- [11. Advanced Networking](#11-advanced-networking)
- [12. UX & UI Polish](#12-ux--ui-polish)
- [13. Deployment & Packaging](#13-deployment--packaging)
- [14. Hardware Buttons](#14-hardware-buttons)

---

## Legend

- [x] Implemented
- [ ] Not yet implemented
- 🐛 Known bug / broken in current implementation
- 🔮 Future / nice-to-have (lower priority)

---

## Implementation Tasks (Sequential)

These tasks define the next end-to-end work items in a deliberate order. When a task is implemented, mark its entry as `[x]` here and also update the matching checkbox in the feature section below.

- [x] Task 1 (12.3) Standardise form pattern for config cards (read-only view + `Edit` button)
- [x] Task 2 (12.2) Network page card grouping into **Status / Configuration / Advanced**
- [x] Task 3 (8) Captive portal: auto-accept portal terms
- [x] Task 4 (7) Authentication: IP-based access control
- [ ] Task 5 (7) Authentication: Two-factor authentication
- [ ] Task 6 (13) Deployment: automatic updates mechanism
- [ ] Task 7 (14) Hardware buttons: long-press vs short-press differentiation
- [ ] Task 8 (14) Hardware buttons: custom button action scripting
- [ ] Task 9 (12) UX: multi-language support (i18n)
- [ ] Task 10 (11) Advanced networking: mDNS / Bonjour forwarding
- [ ] Task 11 (11) Advanced networking: VLAN configuration
- [ ] Task 12 (11) Advanced networking: custom routing rules
- [ ] Task 13 (6) Monitoring: historical data (store + display recent hours/days)
- [ ] Task 14 (2.7) Network management: priority-based WAN failover (health check + auto-switch + notification)
- [ ] Task 15 (2.5) USB/BT: Bluetooth tethering
- [ ] Task 16 (1.3) WiFi modes: Mesh / WDS mode
- [ ] Task 17 (1.4) Multi-radio: startup script to auto-discover radio setup and persist config
- [ ] Task 18 (2.2) Clients: bandwidth limiting (QoS per device)
- [ ] Task 19 (2.2) Clients: parental controls / client group policies
- [ ] Task 20 (3.3) VPN: VPN speed test
- [ ] Task 21 (3.3) VPN: OpenVPN support
- [ ] Task 22 (4.2) AdGuard: blocklist management from the travel router UI
- [ ] Task 23 (4.5) Dynamic DNS: custom DDNS update URL
- [ ] Task 24 (4.6) Future services: cloudflared (Cloudflare Tunnel)
- [ ] Task 25 (4.6) Future services: SQM / QoS (traffic shaping)
- [ ] Task 26 (4.6) Future services: Watchcat (connection watchdog)

---

## 0. Bug List / smaller changes

- [x] AdguardConfig button under Services: {"error":"reading AdGuard config: open /opt/AdGuardHome/AdGuardHome.yaml: no such file or directory"}
- [x] request fails http://192.168.1.1/api/v1/system/timezone: {"error":"uci show system.system: uci: Entry not found"}
- [x] Seeing in VPN view when wireguard is installed error calls like: {"error":"wg show failed: exit status 1"}
- [x] Toggles should also be bluish in dark mode (currently they are grey)
- [x] Links should be moved from services to system
- [x] Link for LUCI is wrong (with this program this is moved to :8080)
- [x] Adguard config viewer / editor does not work
- [x] VPN Overview in VPN tab is not useful. Remove
- [x] Why is startup of this app logged as err in the logs? (errThu Mar 12 12:27:11 2026 daemon.err openwrt-travel-gui[12344]: 2026/03/12 12:27:11 Starting openwrt-travel-gui backend on :80 (mock=false))
- [x] Wireless apply must keep LuCI-style rollback semantics: backend may start `uci apply` with rollback, but must not self-confirm immediately. Confirmation must come only after the frontend/browser proves the router is still reachable on the new WiFi settings.
- [x] WiFi connect must normalize any reused STA section to `network=wwan` whenever the option is missing or not `wwan`, and must fail loudly if creating `network.wwan` or attaching it to the firewall `wan` zone fails. "Associated to upstream WiFi but no DHCP/internet" is not acceptable.
- [x] WiFi mode switching must map UI modes (`client`, `ap`, `repeater`) to valid OpenWrt UCI structures. Writing invalid values like `mode=client` or `mode=repeater` into a `wifi-iface` section is forbidden; repeater mode must be modeled as STA + AP config, not a single iface mode string.
- [x] Repeater wizard must preserve upstream band/radio choice and configure the intended AP sections explicitly on multi-radio hardware. Using only the first AP section is not sufficient for dual-band devices.
- [x] AP, guest WiFi, and STA MAC update flows must return an error when runtime apply fails. Do not commit wireless changes, report success, and leave the device in a "saved in UCI but not actually active" state.
- [x] WiFi/AP management must discover radios and AP sections dynamically instead of assuming `radio0`, `radio1`, `default_radio0..3`, or guest WiFi on `radio0`, so the UI still works after LuCI resets or on different hardware layouts.
- [x] 🐛 WireGuard should work out-of-the-box after service install + profile import/activate: runtime `wg0` must be created and status endpoints must not return success when tunnel is not actually up. See [WireGuard + AdGuard Out-Of-Box Fix Plan](../wireguard-adguard-oob-fix-plan.md).
- [x] 🐛 AdGuard should work out-of-the-box after install + DNS enable: backend install/running detection and DNS forwarding path must be functionally valid (no `nslookup` timeout after successful toggle). See [WireGuard + AdGuard Out-Of-Box Fix Plan](../wireguard-adguard-oob-fix-plan.md).
- [x] 🐛 WiFi page can incorrectly show "Repeater" as active when device is not actually in repeater mode; active mode indicator must reflect real UCI/runtime state.
- [x] 🐛 Saved STA networks must persist and not be removed unexpectedly; only the selected/active network should be connected at runtime while other saved profiles remain stored.
- [x] 🐛 Active WiFi network badge can display lock icon with `unknown` encryption/type; UI should show correct security label based on scan/connection metadata.
- [x] Add machine-readable API documentation generation (OpenAPI/Swagger) and expose docs endpoint from backend for agent/test automation use.
- [x] Document in `AGENTS.md` that the test device can expose/serve API docs endpoints for automation flows.
- [x] 🐛 WireGuard: disabling WireGuard must restore internet connectivity (default route) and the toggle API must accept `enabled` (not only legacy `enable`). See [WireGuard disable breaks internet (plan)](../plans/2026-03-26-wireguard-disable-breaks-internet.md).
- [ ] 🐛 WiFi/VPN UI does not live-update after actions (connect/toggle/apply) — WiFi tab and VPN views can remain stale until manual reload. Fix state invalidation and/or add event-driven refresh. See [Live state refresh + WiFi saved network persistence plan](../plans/2026-03-26-live-state-refresh-and-wifi-saved-networks.md).
- [ ] 🐛 WiFi: connecting to a new upstream AP causes previously saved upstream networks to disappear (profiles not persisted or accidentally deleted). See [Live state refresh + WiFi saved network persistence plan](../plans/2026-03-26-live-state-refresh-and-wifi-saved-networks.md).
- [ ] VPN: verify whether disable latency is required; measure and remove avoidable blocking where possible. See [VPN disable latency + DNS forwarding plan](../plans/2026-03-26-vpn-disable-latency-and-dns-forwarding.md).
- [ ] VPN: DNS forwarding should use VPN-configured DNS servers when VPN is enabled, and restore previous DNS forwarding when VPN is disabled. See [VPN disable latency + DNS forwarding plan](../plans/2026-03-26-vpn-disable-latency-and-dns-forwarding.md).

## 1. WiFi Management

### 1.1 Upstream WiFi (STA / WWAN — Connect to existing WiFi)

- [x] Scan for available networks (SSID, signal strength, encryption, frequency band)
- [x] Connect to a selected network with password
- [x] Show current upstream WiFi connection status (SSID, signal, IP)
- [x] List saved/configured WiFi networks
- [x] Signal strength visualization (icon + dBm + percentage)
- [x] Security badge (WPA2, WPA3, Open, etc.)
- [x] Scan dialog (button-triggered popup with results)
- [x] Hover tooltips on scan results (signal dBm, channel, band, encryption, BSSID)
- [x] Disconnect from upstream WiFi
- [x] Delete saved WiFi networks
- [x] Priority ordering of saved networks (auto-connect preference)
- [x] Auto-reconnect to known networks when connection drops
- [x] Hidden network support (manual SSID entry)
- [x] **Dual-band scan bundling** — Show one row per SSID when the same network is advertised on 2.4 GHz and 5 GHz (macOS-style), with band picker on connect and per-band signal display. See [WiFi dual-band bundling & band switching (plan)](../plans/wifi-dual-band-bundling.md).
- [x] **Automatic band switching** — Background monitor switches STA between 2.4 GHz and 5 GHz based on signal quality with configurable thresholds and hysteresis. See [WiFi dual-band bundling & band switching (plan)](../plans/wifi-dual-band-bundling.md#part-3-automatic-band-switching).

### 1.2 Access Point (AP — Own WiFi for clients)

- [x] Configure AP SSID and password
- [x] Separate 2.4 GHz and 5 GHz AP networks (or combined, depending on hardware)
- [x] Show which radios are available on the device and their capabilities
- [x] Enable/disable AP per radio
- [x] Guest network with client isolation
- [x] QR code for WiFi sharing (generate scannable QR with AP credentials)
- [x] At startup, ensure enabled AP sections have a valid SSID and key (health check fixes missing values, skips disabled APs to avoid ath11k driver crashes). Startup health commits repairs but does not auto-apply wireless changes that would require browser confirmation; the user applies them via LuCI Save & Apply or reboot. User-driven wireless changes use rpcd `uci apply` rollback with explicit browser confirmation. Auto-reconnect cron script still uses `wifi up`.
- [x] AP disable with confirmation warnings (STA deletable without warning). See [Implementation guide](../plans/implementation.md#12--ap-disable-with-warnings).
- [x] 🔮 Band steering (prefer 5 GHz when client supports it)
- [x] 🔮 Scheduled WiFi (time-based on/off)

### 1.3 WiFi Modes

- [x] WiFi mode switching API (AP / STA / Repeater)
- [x] Clear UI for mode selection with explanation of each mode
- [x] Repeater mode setup wizard (upstream + AP on same radio)
- [ ] 🔮 Mesh / WDS mode

### 1.4 Multi-Radio Support

- [x] Detect and enumerate all radio hardware (phy0, phy1, …)
- [x] Per-radio configuration (one for uplink, one for AP, etc.). See [Implementation guide](../plans/implementation.md#14--multi-radio-per-radio-config).
- [x] Recommended configuration based on detected hardware
- [ ] 🔮 Startup script to auto-discover radio setup and persist config

---

## 2. Network Management

### 2.1 Interfaces & Status

- [x] Show WAN, LAN, WWAN interface status (IP, MAC, RX/TX bytes)
- [x] Internet reachability detection
- [x] WAN configuration (static IP, DHCP, PPPoE)
- [x] WAN settings: IP, netmask, gateway, DNS, MTU
- [x] WAN/WWAN interplay explanation (which is active, failover?)
- [x] Interface up/down toggle
- [x] Per-interface real-time traffic charts (RX/TX throughput via WebSocket)

### 2.2 Connected Clients

- [x] List DHCP clients (MAC, IP, hostname)
- [x] Fix "connected since" showing invalid date 🐛
- [x] Device aliases (name clients: "John's Phone", "Laptop" — stored in local config)
- [x] Set static IP / DHCP reservation for a client
- [x] Traffic stats per client (RX/TX)
- [x] Client hostname resolution (DHCP leases + /etc/hosts cross-reference)
- [x] Block / kick a client
- [x] Dedicated **Clients** tab/page with structured per-client view (device identity, connection type/path, interface, signal/IP/MAC, activity), plus quick actions (set hostname alias, add static IP reservation, block/ban, unblock/kick). Should consolidate existing client actions into one focused workflow.
- [ ] 🔮 Client bandwidth limiting (QoS per device)
- [ ] 🔮 Parental controls / client group policies

### 2.3 DHCP & DNS

- [x] Configure DHCP range (start, end, lease time)
- [x] View active DHCP leases with expiry
- [x] Custom DNS servers for LAN
- [x] Local DNS entries (hostname → IP mapping)
- [x] 🔮 DNS over HTTPS / DNS over TLS toggle

### 2.4 Data Usage Tracking

- [x] Track cumulative RX/TX per WAN source (Ethernet, WiFi, USB Tether). See [Data Usage Tracking plan](../plans/data-usage-tracking.md).
- [x] Data usage budget / cap with warning threshold. See [Data Usage Tracking plan](../plans/data-usage-tracking.md).
- [x] Reset counters (per-session, daily, manual). See [Data Usage Tracking plan](../plans/data-usage-tracking.md).
- [x] Show data usage on dashboard (cumulative RX/TX since boot, human-readable format)

### 2.5 USB Tethering

- [x] Detect USB tethered device (phone sharing mobile data). See [USB Tethering plan](../plans/usb-tethering.md).
- [x] Auto-configure as WAN source. See [USB Tethering plan](../plans/usb-tethering.md).
- [x] Show tethering status on dashboard. See [USB Tethering plan](../plans/usb-tethering.md).
- [ ] 🔮 Bluetooth tethering

### 2.6 WAN Auto-Detection

- [x] WAN connection type auto-detection (DHCP, PPPoE, static)
- [x] Detect ethernet cable plug-in and auto-switch to wired WAN. See [Implementation guide](../plans/implementation.md#26--detect-ethernet-cable-plug-in).
- [x] Show active WAN source indicator on dashboard

### 2.7 Connection Failover

- [ ] 🔮 Priority-based WAN source (Ethernet > WiFi > USB Tether). See [Connection Failover plan](../plans/connection-failover.md).
- [ ] 🔮 Health check via periodic ping (configurable target). See [Connection Failover plan](../plans/connection-failover.md).
- [ ] 🔮 Auto-switch to next source on failure. See [Connection Failover plan](../plans/connection-failover.md).
- [ ] 🔮 Notification on failover event. See [Connection Failover plan](../plans/connection-failover.md).

---

## 3. VPN Management

### 3.1 WireGuard

- [x] View WireGuard configuration (peers, keys, endpoints)
- [x] Edit WireGuard configuration
- [x] Import from .conf file (full parser with key validation)
- [x] Enable / disable WireGuard
- [x] Show connection status (handshake time, transfer stats)
- [x] Multiple WireGuard profiles (save, switch, delete VPN configurations)
- [x] Kill switch (block traffic if VPN drops)
- [x] Also install luci-proto-wireguard for wireguard. See [Implementation guide](../plans/implementation.md#31--install-luci-proto-wireguard).
- [x] VPN needs own interfaces, zones, rules. See [WireGuard Full Networking plan](../plans/wireguard-full-networking.md).
- [x] Verify button to check VPN config (interfaces, zones, routes). See [WireGuard Full Networking plan](../plans/wireguard-full-networking.md#phase-3--verify-vpn-button).
- [x] Split tunneling (route only selected traffic through VPN). See [WireGuard Full Networking plan](../plans/wireguard-full-networking.md#phase-5--split-tunneling-future).
- [x] VPN + AdGuard interplay configuration. See [AdGuard Auto-Configure plan](../plans/adguard-auto-configure.md#phase-3--vpn--adguard-interplay).

### 3.2 Tailscale

- [x] Status endpoint (stub — returns not installed)
- [x] Toggle endpoint (stub)
- [x] Actual Tailscale integration (login, device list, exit node). See [Tailscale Integration plan](../plans/tailscale-integration.md).
- [x] Show Tailscale IP and connected peers. See [Tailscale Integration plan](../plans/tailscale-integration.md#phase-2--status--connected-peers).
- [x] Exit node selection. See [Tailscale Integration plan](../plans/tailscale-integration.md#phase-3--exit-node-selection).
- [x] 🔮 Tailscale SSH toggle

### 3.3 General VPN UX

- [x] Grey out VPN options when packages not installed (link to Services page)
- [x] Backend loads installed-service state at startup; UI reads cached state (no dynamic per-page checks)
- [x] Show VPN data usage on dashboard. See [Implementation guide](../plans/implementation.md#33--vpn-data-usage-on-dashboard).
- [x] DNS leak test (router-side: WireGuard DNS vs effective upstream — `/etc/resolv.conf` plus dnsmasq `server=` when resolv only lists the local stub). With VPN DNS configured, WireGuard enable **replaces** dnsmasq forwards with those IPs (previous list, e.g. AdGuard `127.0.0.1#5353`, is snapshotted and restored on VPN disable). See [Implementation guide](../plans/implementation.md#33--dns-leak-test).
- [ ] Upload VPN profiles via file upload (UI) in addition to copy/paste (WireGuard `.conf` at minimum; future-proof for OpenVPN).
- [ ] 🔮 OpenVPN support
- [ ] 🔮 VPN speed test

---

## 4. Services Management

### 4.1 Service Lifecycle

- [x] List services with install state (not installed / installed / running / stopped)
- [x] Install packages (APK or opkg auto-detection)
- [x] Remove packages
- [x] Start / stop services via init.d
- [x] Install progress feedback — streaming log output in a popup/dialog so user sees what's happening
- [x] Show install errors clearly
- [x] Service auto-start on boot toggle
- [x] Backend caches installed-service state at startup; updates on install/remove actions

### 4.2 AdGuard Home

- [x] Status check (installed, running, version)
- [x] Query statistics (total, blocked, percentage, avg response time)
- [x] Auto-configure after install (port, interfaces, DNS integration with dnsmasq). See [AdGuard Auto-Configure plan](../plans/adguard-auto-configure.md).
- [x] "AdGuard Home handles client requests" toggle (GL.iNet style, with VPN hint; defaults to off)
- [x] Show if AdGuard is configured as default DNS for LAN
- [x] Quick link to AdGuard web UI (with correct IP:port)
- [x] Toggle DNS filtering on/off without stopping AdGuard
- [x] AdGuard Home configuration editor (show/edit AdGuardHome.yaml with restart)
- [x] Configure AdGuard to work alongside VPN. See [AdGuard Auto-Configure plan](../plans/adguard-auto-configure.md#phase-3--vpn--adguard-interplay).
- [ ] 🔮 Blocklist management from travel router UI

### 4.3 WireGuard (as service)

- [x] Install wireguard-tools package
- [x] Post-install setup wizard. See [Implementation guide](../plans/implementation.md#43--wireguard-post-install-wizard).

### 4.4 Tailscale (as service)

- [x] Install tailscale package
- [x] Post-install authentication flow. See [Implementation guide](../plans/implementation.md#44--tailscale-post-install-auth-flow) and [Tailscale Integration plan](../plans/tailscale-integration.md#phase-1--authentication-flow).
- [ ] UI navigation: move Tailscale out of the VPN page into a Services sub-menu (Services → Tailscale).

### 4.5 Dynamic DNS

- [x] DDNS provider configuration (DuckDNS, No-IP, Cloudflare, etc.)
- [x] DDNS status indicator (current public IP, last update)
- [ ] 🔮 Custom DDNS update URL

### 4.6 Future Services

- [ ] 🔮 Cloudflared (Cloudflare Tunnel)
- [ ] 🔮 SQM / QoS (traffic shaping)
- [ ] 🔮 Watchcat (connection watchdog)

---

## 5. System Management

### 5.1 System Information

- [x] Hardware model, firmware version, kernel version
- [x] Hostname
- [x] Uptime
- [x] CPU usage, load averages, core count
- [x] Memory usage (total, used, free, cached)
- [x] Storage usage (total, used, free)
- [x] Optional: CPU temperature

### 5.2 System Actions

- [x] Reboot with confirmation dialog
- [x] Reboot actually working on device
- [x] Shutdown button. See [Implementation guide](../plans/implementation.md#52--mark-reboot-as-working--add-shutdown).
- [x] Firmware upgrade (upload sysupgrade image)
- [x] Factory reset with confirmation
- [x] Hostname change
- [x] Backup / restore configuration (export/import UCI configs as archive)
- [x] LED control — stealth mode (turn off all router LEDs via sysfs toggle)
- [x] LED control — per-LED brightness display and scheduled on/off via cron

### 5.3 Time & Timezone

- [x] Show current device time
- [x] Timezone configuration
- [x] Detect timezone mismatch between device and browser (GL.iNet style)
- [x] NTP server configuration
- [x] Browser time sync on login — before JWT is issued, client POSTs its clock to `/api/v1/system/time-sync`; if skew > 60s the router clock is corrected via `date -s` (fixes clock-skew login loop on devices without NTP access at first boot)
- [x] Auto-set timezone from browser on mismatch alert click. See [Implementation guide](../plans/implementation.md#53--auto-set-timezone-from-browser).
- [x] NTP manual sync button + travel timezone validation. See [Implementation guide](../plans/implementation.md#53--time-sync-travel-validation).
- [x] 🔮 NTP sync status indicator

### 5.4 Password Management

- [x] Login with password (bcrypt hashed)
- [x] Change admin password from UI
- [x] Password strength requirements
- [x] 🔮 SSH key management

---

## 6. Dashboard & Monitoring

### 6.1 Status Cards

- [x] Connection status (WAN/WiFi, signal, IP)
- [x] VPN status card
- [x] System stats (CPU, memory, uptime)
- [x] Connected clients count
- [x] Captive portal detection banner

### 6.2 Real-Time Monitoring

- [x] WebSocket-based live system stats (2s interval)
- [x] Bandwidth chart (CPU/Memory over time, 15 data points)
- [x] Network throughput chart (RX/TX bytes/sec)
- [x] Per-interface traffic chart
- [x] Connection uptime log (internet available since / lost at — timeline of events). See [Implementation guide](../plans/implementation.md#62--connection-uptime-log).
- [ ] 🔮 Historical data (store and display last hours/days)

### 6.3 Quick Actions

- [x] WiFi scan shortcut
- [x] Reboot shortcut
- [x] VPN toggle shortcut
- [x] WiFi on/off shortcut

### 6.4 Notifications & Alerts

- [x] Push alerts via WebSocket (storage low, high CPU, high memory)
- [x] Notification bell with unread count badge in header
- [x] Dropdown panel showing recent alerts with severity indicators
- [x] Toast notification on new alert
- [x] Notification history via GET /api/v1/system/alerts (last 50 in memory)
- [x] 🔮 Configurable alert thresholds (e.g., storage < 10%)

---

## 7. Authentication & Security

- [x] Password-based authentication with bcrypt
- [x] JWT tokens (24h expiry)
- [x] Token revocation via blocklist
- [x] Rate limiting (5 failed attempts/min)
- [x] Remember me toggle (localStorage vs sessionStorage)
- [x] WebSocket auth via JWT query parameter
- [x] Session timeout warning (toast 5 min before JWT expiry, auto-redirect on expiry)
- [x] HTTPS / TLS support (low priority). See [Implementation guide](../plans/implementation.md#7--https--tls-support).
- [ ] 🔮 Two-factor authentication
- [x] IP-based access control (env `ALLOWED_ADMIN_CIDRS` / `--allowed-admin-cidrs`; loopback always allowed; exempt: `/api/health`, `/api/v1/auth/login`, `/api/v1/system/time-sync`)

---

## 8. Captive Portal

- [x] Auto-detection via HTTP connectivity check (Google 204 probe)
- [x] Detect redirect URL for portal login
- [x] Banner on dashboard and WiFi page
- [x] Auto-refresh every 30 seconds
- [x] Open portal login in new tab (button in captive portal banner)
- [x] Auto-accept portal terms (common portal patterns)

---

## 9. Logs & Diagnostics

- [x] System log viewer (logread / syslog)
- [x] Kernel log (dmesg)
- [x] Service-specific logs (AdGuard, WireGuard, etc.)
- [x] Log level filtering
- [x] Log search / filter
- [x] Log export / download
- [x] 🔮 Network diagnostics (ping, traceroute, DNS lookup from device)
- [x] 🔮 Speed test (run from device to measure actual WAN throughput)

---

## 10. Privacy & Identity

- [x] MAC address cloning (copy client MAC for hotel WiFi device registration)
- [x] MAC address randomization / anonymization (generate random MAC per connection)
- [x] Show current MAC per interface
- [x] 🔮 Per-network MAC policy (remember which MAC to use for which SSID)

---

## 11. Advanced Networking

- [ ] 🔮 mDNS / Bonjour forwarding (Chromecast, AirPlay across network segments)
- [x] 🔮 Firewall zone summary (WAN/LAN/VPN zone overview — not full rule editor)
- [x] 🔮 Port forwarding
- [ ] 🔮 Custom routing rules
- [ ] 🔮 VLAN configuration
- [x] 🔮 IPv6 support toggle and status
- [x] 🔮 Wake-on-LAN

---

## 12. UX & UI Polish

- [x] Responsive layout (desktop sidebar, mobile drawer)
- [x] Dark mode support
- [x] Loading skeletons
- [x] Error handling with toast notifications
- [x] Tooltips / hover info for WiFi networks (signal details, channel, etc.)
- [x] Tooltips for technical fields (what is MTU? what is DHCP range?). See [Implementation guide](../plans/implementation.md#12--tooltips-for-technical-fields).
- [x] Onboarding / first-run setup wizard
- [x] Connection status indicator in header (green/red dot)
- [x] Actions dropdown menu in toolbar (Reboot, Logout)
- [x] Quick links to LuCI and AdGuard dashboards on Services page
- [x] Dialog close button visible in dark mode
- [x] Select component dark mode styling
- [ ] 🔮 Multi-language support (i18n)
- [x] 🔮 PWA enhancements (offline indicator, app-like experience)

### 12.2 Information Architecture & Page Structure

> The UI currently exposes every feature as a flat list of cards on each page.
> Non-technical users face 10–22 cards per page with no grouping, collapsing,
> or progressive disclosure. The changes below prioritise simplicity for daily
> use while keeping power-user features accessible behind sub-menus or
> expandable sections. See [UX Overhaul plan](../plans/ux-overhaul.md) for
> full design rationale and implementation details.

#### Network page (21 cards, 1008 lines — most complex page)

- [x] Group cards into tabbed/collapsible sections: **Status** (connectivity, WAN source, traffic charts, connected clients), **Configuration** (WAN, LAN, DHCP, DNS), **Advanced** (DDNS, firewall, port forwarding, IPv6, DoH, WoL, diagnostics, speed test, USB tethering). See [UX Overhaul plan](../plans/ux-overhaul.md#network-page-restructure).
- [x] Move Connected Clients card from position 13 to the Status group (top of page)
- [x] Merge Internet Connectivity badge into WAN Source card (save one full card for a single badge)
- [x] Make firewall port-forward add-form responsive (currently hard 6-column grid breaks on mobile). See [UX Overhaul plan](../plans/ux-overhaul.md#mobile-form-fixes).
- [x] Make DHCP reservation / DNS entry add-forms responsive (3–4 column grids with no breakpoints)
- [x] Extract network-page.tsx inline sections into separate components (target: page file under 300 lines). See [UX Overhaul plan](../plans/ux-overhaul.md#network-page-extraction).

#### System page (13+ cards, 1087 lines — second most complex)

- [x] Reorder cards by frequency: **At-a-glance** (System Info + Uptime merged, Stats) at top, **Configuration** (Timezone, NTP, Password, SSH Keys, LED, Buttons, Alert Thresholds) in middle, **Danger Zone** (Firmware Upgrade, Factory Reset, Reboot, Shutdown) at bottom with a visual separator. See [UX Overhaul plan](../plans/ux-overhaul.md#system-page-restructure).
- [x] Merge Uptime into System Information card (Uptime is currently a single-line standalone card)
- [x] Move Quick Links from first position to bottom utility section
- [x] Replace `window.confirm()` in Restore with a proper Dialog (only place in the app using native confirm)
- [x] Make Reboot/Shutdown use full Dialog confirmation (currently inline badge + 2 small buttons — weaker than Factory Reset's modal)
- [x] Replace raw `<input type="checkbox">` with `<Switch>` component for NTP enable and Firmware keep-settings toggles (inconsistent with LED schedule toggle)
- [x] Extract large inline sections into separate components (LED 132 lines, Actions 113 lines, Firmware 103 lines, NTP 97 lines). See [UX Overhaul plan](../plans/ux-overhaul.md#system-page-extraction).

#### WiFi page (10 cards, 572 lines)

- [x] Group bottom 5 cards (Guest, MAC Address, MAC Policy, Band Switching, Schedule) into a collapsible "Advanced WiFi Settings" section — these are set-once features. See [UX Overhaul plan](../plans/ux-overhaul.md#wifi-page-restructure).
- [x] Hide mode-irrelevant sections: hide "Saved Networks" in pure AP mode, hide "Access Point Configuration" in pure STA mode
- [x] Extract inline cards (Radio Hardware, Current Connection, Saved Networks, AP Config) into separate component files (currently 442 lines inline in wifi-page.tsx)

#### Dashboard

- [x] Move QuickActions above the charts (currently below fold, hard to discover). See [UX Overhaul plan](../plans/ux-overhaul.md#dashboard-improvements).
- [x] Fix QuickActions label: "WiFi On" / "WiFi Off" describes state, not action — change to "Disable WiFi" / "Enable WiFi"
- [x] Remove duplicate feedback: QuickActions uses inline ActionState icons AND hook-level Sonner toasts simultaneously — pick one
- [x] Remove `window.confirm()` from QuickActions Reboot — use Dialog component (matches header pattern)
- [x] Fix chart hardcoded hex colors for dark mode (tooltip bg `rgba(0,0,0,0.8)`, grid lines `#9ca3af`). See [UX Overhaul plan](../plans/ux-overhaul.md#dark-mode-chart-fix).
- [x] Show router hostname/model in header for multi-device disambiguation

### 12.3 Consistency & Polish

- [x] Standardise confirmation UX: all destructive actions (Reboot, Shutdown, Factory Reset, Firmware Upgrade, Restore) must use the Dialog component with explicit warning text — no `window.confirm()`, no inline badge-based confirmations. See [UX Overhaul plan](../plans/ux-overhaul.md#confirmation-standardisation).
- [x] Standardise form pattern: configuration cards should have a read-only view state with an Edit button, instead of permanently showing editable forms (Timezone, NTP, Password, Hardware Buttons are always in edit mode)
- [x] Standardise empty states: use a consistent "no data" component across all cards instead of ad-hoc `<p className="text-sm text-gray-500">` strings
- [x] Ensure `SystemStatsCard` shows a Skeleton on error instead of returning `null` (which collapses the grid cell)
- [ ] Long-running enable/disable actions (VPN toggle, WiFi connect/apply, service start/stop): use a modal/popup progress UI with clearer “what is happening” details and elapsed time; keep the current page responsive while the operation runs.

---

## 13. Deployment & Packaging

- [x] IPK packaging for OpenWRT
- [x] Build scripts (build.sh, deploy.sh)
- [x] Install / uninstall scripts
- [x] APK + opkg package manager auto-detection
- [x] `setup-local.sh` — one-time fresh-device setup over SSH (moves LuCI to :8080, uploads init.d + UCI config, marks setup complete)
- [x] AP WiFi health check at startup (ensures enabled APs have valid config; skips disabled APs)
- [x] Init script for auto-start on boot (procd service, verified on device)
- [x] CI/CD pipeline (build + test + package). See [CI/CD Pipeline plan](../plans/cicd-pipeline.md).
- [x] Size optimization audit (bundle size, Go binary strip). See [Implementation guide](../plans/implementation.md#13--size-optimization-audit).
- [ ] 🔮 Automatic updates mechanism

---

## 14. Hardware Buttons

- [x] Detect hardware buttons (identify available buttons via /etc/rc.button/). See [Hardware Buttons plan](../plans/hardware-buttons.md).
- [x] Configure button actions (VPN toggle, WiFi on/off, LED toggle, etc.). See [Hardware Buttons plan](../plans/hardware-buttons.md#phase-2--configure-button-actions).
- [x] Button event handler via hotplug.d integration. See [Hardware Buttons plan](../plans/hardware-buttons.md).
- [ ] 🔮 Custom button action scripting
- [ ] 🔮 Long-press vs short-press differentiation. See [Hardware Buttons plan](../plans/hardware-buttons.md#phase-4--long-press-vs-short-press-future).

---

## 15. VPN, Wi‑Fi stability, SQM, performance & AdGuard (field issues)

- [x] **WireGuard enable/error/runtime**: Enabling VPN from the UI shows an error while `wg0` still appears in LuCI; traffic does not use the tunnel. Align backend apply order with netifd/LuCI (interface, peer, firewall, routing). Ensure UCI objects LuCI expects exist before enable. Harden verification (avoid failing the whole operation when the interface is up but a handshake is still in progress).
- [ ] **Single active VPN policy**: Only one VPN-style path may be active at a time across WireGuard, Tailscale (exit node / full-tunnel style use), and future providers. Enabling one must disable or demote others deterministically.
- [x] **Wi‑Fi SSID/password visibility after change**: After changing AP name/password via the GUI, clients do not see the network until the SSID is toggled in LuCI. Wireless confirm/apply path should fully restart or reload hostapd so the BSS reappears without manual LuCI steps.
- [ ] **SQM / traffic shaping**: Offer SQM install (cake/fq_codel) with defaults suitable for most travel-router WANs; optional presets per scenario. Reference: [OpenWrt SQM](https://openwrt.org/docs/guide-user/network/traffic-shaping/sqm).
- [x] **Backend CPU usage**: Investigate sustained high CPU from `openwrt-travel-gui` (e.g. WebSocket/ubus work while no clients are connected, service restart behaviour). Add safeguards: back off when idle, avoid redundant hot paths, document intentional monitors.
- [x] **AdGuard Home admin URLs from YAML**: Load bind host/port (and related listen settings) from `/etc/adguardhome/adguardhome.yaml` at backend startup so the UI shows correct links instead of hard-coded assumptions.
- [x] **Attended Sysupgrade preferences**: On install/setup set `uci set attendedsysupgrade.client.login_check_for_upgrades='1'` and commit so update checks follow project policy.

---

## Performance & Size Constraints

> Router devices have limited CPU, RAM (128–512 MB), and storage (16–128 MB flash).
> Every dependency and feature must justify its footprint.

- Go backend: single static binary, no CGO, stripped symbols
- Frontend: Vite tree-shaking, code-splitting per route, minimal dependencies
- No heavy JS frameworks beyond React core
- Images: SVG only, no raster assets
- API: minimal JSON payloads, avoid polling where WebSocket exists
- Package installs: warn user about storage impact before installing

---

## Notes & Open Questions

1. **Multi-radio strategy**: What's the recommended radio assignment for travel routers with 2+ radios? (e.g., radio0=5GHz AP, radio1=2.4GHz STA uplink) — needs hardware-specific discovery.
2. **WAN vs WWAN**: How should they coexist? Does WWAN override WAN? Is metric-based routing sufficient?
3. **Startup safety net**: Should the install script always ensure an AP is broadcasting so the user can never lock themselves out?
4. **AdGuard DNS setup**: On install, should we move dnsmasq to port 5353 and let AdGuard take port 53? Or configure AdGuard to forward to dnsmasq?
5. **Repeater mode implications**: In repeater mode, a single radio handles both STA and AP — significant throughput reduction. Should we warn the user?
6. **GL.iNet feature parity targets**: Multi-WAN failover, VPN policies (per-client VPN routing), Goodcloud-like remote management — which are realistic for this project?
