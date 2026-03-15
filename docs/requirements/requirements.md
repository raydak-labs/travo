# OpenWRT Travel Router GUI — Feature Requirements

> **Last updated:** 2026-03-15 (v28 — Bug fixes, implementation plans, feature categorization)

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
- [ ] **Dual-band scan bundling** — Show one row per SSID when the same network is advertised on 2.4 GHz and 5 GHz (macOS-style), with optional "prefer 5 GHz" for the STA. See [WiFi dual-band bundling (plan)](../plans/wifi-dual-band-bundling.md).

### 1.2 Access Point (AP — Own WiFi for clients)

- [x] Configure AP SSID and password
- [x] Separate 2.4 GHz and 5 GHz AP networks (or combined, depending on hardware)
- [x] Show which radios are available on the device and their capabilities
- [x] Enable/disable AP per radio
- [x] Guest network with client isolation
- [x] QR code for WiFi sharing (generate scannable QR with AP credentials)
- [x] At startup, ensure enabled AP sections have a valid SSID and key (health check fixes missing values, skips disabled APs to avoid ath11k driver crashes). We use `wifi up` only when an AP was re-enabled; SSID/key-only fixes are committed without running wifi. All wireless apply uses `wifi up`, not `wifi reload`, to avoid ath11k/IPQ6018 crash loops. Auto-reconnect cron script also uses `wifi up`.
- [ ] AP disable with confirmation warnings (STA deletable without warning). See [Implementation guide](../plans/implementation.md#12--ap-disable-with-warnings).
- [ ] 🔮 Band steering (prefer 5 GHz when client supports it)
- [ ] 🔮 Scheduled WiFi (time-based on/off)

### 1.3 WiFi Modes

- [x] WiFi mode switching API (AP / STA / Repeater)
- [x] Clear UI for mode selection with explanation of each mode
- [x] Repeater mode setup wizard (upstream + AP on same radio)
- [ ] 🔮 Mesh / WDS mode

### 1.4 Multi-Radio Support

- [x] Detect and enumerate all radio hardware (phy0, phy1, …)
- [ ] Per-radio configuration (one for uplink, one for AP, etc.). See [Implementation guide](../plans/implementation.md#14--multi-radio-per-radio-config).
- [ ] Recommended configuration based on detected hardware
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
- [ ] 🔮 Client bandwidth limiting (QoS per device)
- [ ] 🔮 Parental controls / client group policies

### 2.3 DHCP & DNS

- [x] Configure DHCP range (start, end, lease time)
- [x] View active DHCP leases with expiry
- [x] Custom DNS servers for LAN
- [x] Local DNS entries (hostname → IP mapping)
- [ ] 🔮 DNS over HTTPS / DNS over TLS toggle

### 2.4 Data Usage Tracking

- [ ] Track cumulative RX/TX per WAN source (Ethernet, WiFi, USB Tether). See [Data Usage Tracking plan](../plans/data-usage-tracking.md).
- [ ] Data usage budget / cap with warning threshold. See [Data Usage Tracking plan](../plans/data-usage-tracking.md).
- [ ] Reset counters (per-session, daily, manual). See [Data Usage Tracking plan](../plans/data-usage-tracking.md).
- [x] Show data usage on dashboard (cumulative RX/TX since boot, human-readable format)

### 2.5 USB Tethering

- [ ] Detect USB tethered device (phone sharing mobile data). See [USB Tethering plan](../plans/usb-tethering.md).
- [ ] Auto-configure as WAN source. See [USB Tethering plan](../plans/usb-tethering.md).
- [ ] Show tethering status on dashboard. See [USB Tethering plan](../plans/usb-tethering.md).
- [ ] 🔮 Bluetooth tethering

### 2.6 WAN Auto-Detection

- [x] WAN connection type auto-detection (DHCP, PPPoE, static)
- [ ] Detect ethernet cable plug-in and auto-switch to wired WAN. See [Implementation guide](../plans/implementation.md#26--detect-ethernet-cable-plug-in).
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
- [ ] Also install luci-proto-wireguard for wireguard. See [Implementation guide](../plans/implementation.md#31--install-luci-proto-wireguard).
- [ ] VPN needs own interfaces, zones, rules. See [WireGuard Full Networking plan](../plans/wireguard-full-networking.md).
- [ ] Verify button to check VPN config (interfaces, zones, routes). See [WireGuard Full Networking plan](../plans/wireguard-full-networking.md#phase-3--verify-vpn-button).
- [ ] Split tunneling (route only selected traffic through VPN). See [WireGuard Full Networking plan](../plans/wireguard-full-networking.md#phase-5--split-tunneling-future).
- [ ] VPN + AdGuard interplay configuration. See [AdGuard Auto-Configure plan](../plans/adguard-auto-configure.md#phase-3--vpn--adguard-interplay).

### 3.2 Tailscale

- [x] Status endpoint (stub — returns not installed)
- [x] Toggle endpoint (stub)
- [ ] Actual Tailscale integration (login, device list, exit node). See [Tailscale Integration plan](../plans/tailscale-integration.md).
- [ ] Show Tailscale IP and connected peers. See [Tailscale Integration plan](../plans/tailscale-integration.md#phase-2--status--connected-peers).
- [ ] Exit node selection. See [Tailscale Integration plan](../plans/tailscale-integration.md#phase-3--exit-node-selection).
- [ ] 🔮 Tailscale SSH toggle

### 3.3 General VPN UX

- [x] Grey out VPN options when packages not installed (link to Services page)
- [x] Backend loads installed-service state at startup; UI reads cached state (no dynamic per-page checks)
- [ ] Show VPN data usage on dashboard. See [Implementation guide](../plans/implementation.md#33--vpn-data-usage-on-dashboard).
- [ ] DNS leak test (verify traffic routes through VPN correctly). See [Implementation guide](../plans/implementation.md#33--dns-leak-test).
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
- [ ] Auto-configure after install (port, interfaces, DNS integration with dnsmasq). See [AdGuard Auto-Configure plan](../plans/adguard-auto-configure.md).
- [x] "AdGuard Home handles client requests" toggle (GL.iNet style, with VPN hint; defaults to off)
- [x] Show if AdGuard is configured as default DNS for LAN
- [x] Quick link to AdGuard web UI (with correct IP:port)
- [x] Toggle DNS filtering on/off without stopping AdGuard
- [x] AdGuard Home configuration editor (show/edit AdGuardHome.yaml with restart)
- [ ] Configure AdGuard to work alongside VPN. See [AdGuard Auto-Configure plan](../plans/adguard-auto-configure.md#phase-3--vpn--adguard-interplay).
- [ ] 🔮 Blocklist management from travel router UI

### 4.3 WireGuard (as service)

- [x] Install wireguard-tools package
- [ ] Post-install setup wizard. See [Implementation guide](../plans/implementation.md#43--wireguard-post-install-wizard).

### 4.4 Tailscale (as service)

- [x] Install tailscale package
- [ ] Post-install authentication flow. See [Implementation guide](../plans/implementation.md#44--tailscale-post-install-auth-flow) and [Tailscale Integration plan](../plans/tailscale-integration.md#phase-1--authentication-flow).

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
- [ ] Shutdown button. See [Implementation guide](../plans/implementation.md#52--mark-reboot-as-working--add-shutdown).
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
- [ ] Auto-set timezone from browser on mismatch alert click. See [Implementation guide](../plans/implementation.md#53--auto-set-timezone-from-browser).
- [ ] NTP manual sync button + travel timezone validation. See [Implementation guide](../plans/implementation.md#53--time-sync-travel-validation).
- [ ] 🔮 NTP sync status indicator

### 5.4 Password Management

- [x] Login with password (bcrypt hashed)
- [x] Change admin password from UI
- [x] Password strength requirements
- [ ] 🔮 SSH key management

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
- [ ] Connection uptime log (internet available since / lost at — timeline of events). See [Implementation guide](../plans/implementation.md#62--connection-uptime-log).
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
- [ ] 🔮 Configurable alert thresholds (e.g., storage < 10%)

---

## 7. Authentication & Security

- [x] Password-based authentication with bcrypt
- [x] JWT tokens (24h expiry)
- [x] Token revocation via blocklist
- [x] Rate limiting (5 failed attempts/min)
- [x] Remember me toggle (localStorage vs sessionStorage)
- [x] WebSocket auth via JWT query parameter
- [x] Session timeout warning (toast 5 min before JWT expiry, auto-redirect on expiry)
- [ ] HTTPS / TLS support (low priority). See [Implementation guide](../plans/implementation.md#7--https--tls-support).
- [ ] 🔮 Two-factor authentication
- [ ] 🔮 IP-based access control

---

## 8. Captive Portal

- [x] Auto-detection via HTTP connectivity check (Google 204 probe)
- [x] Detect redirect URL for portal login
- [x] Banner on dashboard and WiFi page
- [x] Auto-refresh every 30 seconds
- [x] Open portal login in new tab (button in captive portal banner)
- [ ] 🔮 Auto-accept portal terms (common portal patterns)

---

## 9. Logs & Diagnostics

- [x] System log viewer (logread / syslog)
- [x] Kernel log (dmesg)
- [x] Service-specific logs (AdGuard, WireGuard, etc.)
- [x] Log level filtering
- [x] Log search / filter
- [x] Log export / download
- [ ] 🔮 Network diagnostics (ping, traceroute, DNS lookup from device)
- [ ] 🔮 Speed test (run from device to measure actual WAN throughput)

---

## 10. Privacy & Identity

- [x] MAC address cloning (copy client MAC for hotel WiFi device registration)
- [x] MAC address randomization / anonymization (generate random MAC per connection)
- [x] Show current MAC per interface
- [ ] 🔮 Per-network MAC policy (remember which MAC to use for which SSID)

---

## 11. Advanced Networking

- [ ] 🔮 mDNS / Bonjour forwarding (Chromecast, AirPlay across network segments)
- [ ] 🔮 Firewall zone summary (WAN/LAN/VPN zone overview — not full rule editor)
- [ ] 🔮 Port forwarding
- [ ] 🔮 Custom routing rules
- [ ] 🔮 VLAN configuration
- [ ] 🔮 IPv6 support toggle and status
- [ ] 🔮 Wake-on-LAN

---

## 12. UX & UI Polish

- [x] Responsive layout (desktop sidebar, mobile drawer)
- [x] Dark mode support
- [x] Loading skeletons
- [x] Error handling with toast notifications
- [x] Tooltips / hover info for WiFi networks (signal details, channel, etc.)
- [ ] Tooltips for technical fields (what is MTU? what is DHCP range?). See [Implementation guide](../plans/implementation.md#12--tooltips-for-technical-fields).
- [x] Onboarding / first-run setup wizard
- [x] Connection status indicator in header (green/red dot)
- [x] Actions dropdown menu in toolbar (Reboot, Logout)
- [x] Quick links to LuCI and AdGuard dashboards on Services page
- [x] Dialog close button visible in dark mode
- [x] Select component dark mode styling
- [ ] 🔮 Multi-language support (i18n)
- [ ] 🔮 PWA enhancements (offline indicator, app-like experience)

---

## 13. Deployment & Packaging

- [x] IPK packaging for OpenWRT
- [x] Build scripts (build.sh, deploy.sh)
- [x] Install / uninstall scripts
- [x] APK + opkg package manager auto-detection
- [x] `setup-local.sh` — one-time fresh-device setup over SSH (moves LuCI to :8080, uploads init.d + UCI config, marks setup complete)
- [x] AP WiFi health check at startup (ensures enabled APs have valid config; skips disabled APs)
- [x] Init script for auto-start on boot (procd service, verified on device)
- [ ] CI/CD pipeline (build + test + package). See [CI/CD Pipeline plan](../plans/cicd-pipeline.md).
- [ ] Size optimization audit (bundle size, Go binary strip). See [Implementation guide](../plans/implementation.md#13--size-optimization-audit).
- [ ] 🔮 Automatic updates mechanism

---

## 14. Hardware Buttons

- [ ] Detect hardware buttons (identify available buttons via /etc/rc.button/). See [Hardware Buttons plan](../plans/hardware-buttons.md).
- [ ] Configure button actions (VPN toggle, WiFi on/off, LED toggle, etc.). See [Hardware Buttons plan](../plans/hardware-buttons.md#phase-2--configure-button-actions).
- [ ] Button event handler via hotplug.d integration. See [Hardware Buttons plan](../plans/hardware-buttons.md).
- [ ] 🔮 Custom button action scripting
- [ ] 🔮 Long-press vs short-press differentiation. See [Hardware Buttons plan](../plans/hardware-buttons.md#phase-4--long-press-vs-short-press-future).

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
