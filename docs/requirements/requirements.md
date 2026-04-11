# OpenWRT Travel Router GUI — Outstanding requirements

> **Hint:** For the full checklist including everything already implemented, see [`requirements_done.md`](./requirements_done.md). This file lists **only open work** (and ongoing notes).

> **Last updated:** 2026-04-11

---

## Legend

- [ ] Not yet implemented  
- 🔮 Future / nice-to-have (lower priority)

---

## Implementation Tasks (Sequential)

Outstanding numbered tasks (see matching sections below for detail):

- [ ] **Task 5** (§7) Authentication: Two-factor authentication  
- [ ] **Task 6** (§13) Deployment: automatic updates mechanism  
- [ ] **Task 7** (§14) Hardware buttons: long-press vs short-press differentiation  
- [ ] **Task 8** (§14) Hardware buttons: custom button action scripting  
- [ ] **Task 9** (§12) UX: multi-language support (i18n)  
- [ ] **Task 10** (§11) Advanced networking: mDNS / Bonjour forwarding  
- [ ] **Task 11** (§11) Advanced networking: VLAN configuration  
- [ ] **Task 12** (§11) Advanced networking: custom routing rules  
- [ ] **Task 13** (§6) Monitoring: historical data (store + display recent hours/days)  
- [ ] **Task 14** (§2.7) Network management: priority-based WAN failover (health check + auto-switch + notification)  
- [ ] **Task 15** (§2.5) USB/BT: Bluetooth tethering  
- [ ] **Task 16** (§1.3) WiFi modes: Mesh / WDS mode  
- [ ] **Task 17** (§1.4) Multi-radio: startup script to auto-discover radio setup and persist config  
- [ ] **Task 18** (§2.2) Clients: bandwidth limiting (QoS per device)  
- [ ] **Task 19** (§2.2) Clients: parental controls / client group policies  
- [ ] **Task 21** (§3.3) VPN: OpenVPN support  
- [ ] **Task 24** (§4.6) Future services: cloudflared (Cloudflare Tunnel)  
- [ ] **Task 26** (§4.6) Future services: Watchcat (connection watchdog)  

---

## 1. WiFi Management

### 1.2 WiFi Mode Invariants (enforced)

The backend enforces the following invariants at every UCI mutation; the frontend
surfaces violations via `GET /api/v1/wifi/health`:

- **At most one enabled STA (`mode=sta`) bound to `network=wwan`.** Netifd can only
  attach one L3 device to the `wwan` interface, so two enabled STAs cause one to
  associate without an IP and no WAN. `validateWirelessConsistency` rejects the
  apply; `SetMode("repeater"|"client")` picks the winner via
  `/etc/travo/wifi-priorities.json` and disables the rest.
- **In repeater mode with ≥2 radios, STA and AP run on different radios.** On
  ath11k/IPQ6018 a single-PHY STA+AP forces the AP to follow the STA channel;
  a failing STA then drags the AP down. `SetMode("repeater")` includes a
  radio-role planner that moves APs off the STA's radio when an alternate radio
  exists. Single-radio hardware is still allowed to coexist (with the known
  throughput trade-off — see Open Question #5).
- **Auto-reconnect script has a failure-count guard.** `wifi-reconnect.sh` caps
  consecutive failures at `MAX_FAIL=5` and clears the counter on any successful
  reconnect. This prevents cron from replaying a broken config forever if the
  rpcd rollback restored a pre-incident bad config.

### 1.3 WiFi Modes

- [ ] 🔮 Mesh / WDS mode  

### 1.4 Multi-Radio Support

- [ ] 🔮 Startup script to auto-discover radio setup and persist config  

---

## 2. Network Management

### 2.2 Connected Clients

- [ ] 🔮 Client bandwidth limiting (QoS per device)  
- [ ] 🔮 Parental controls / client group policies  

### 2.5 USB Tethering

- [ ] 🔮 Bluetooth tethering  

### 2.7 Connection Failover

- [ ] 🔮 Priority-based WAN source (Ethernet > WiFi > USB Tether). See [Connection Failover plan](../plans/connection-failover.md).  
- [ ] 🔮 Health check via periodic ping (configurable target). See [Connection Failover plan](../plans/connection-failover.md).  
- [ ] 🔮 Auto-switch to next source on failure. See [Connection Failover plan](../plans/connection-failover.md).  
- [ ] 🔮 Notification on failover event. See [Connection Failover plan](../plans/connection-failover.md).  

---

## 3. VPN Management

### 3.3 General VPN UX

- [ ] 🔮 OpenVPN support  

---

## 4. Services Management

### 4.6 Future Services

- [ ] 🔮 Cloudflared (Cloudflare Tunnel)  
- [ ] 🔮 Watchcat (connection watchdog)  

---

## 6. Dashboard & Monitoring

### 6.2 Real-Time Monitoring

- [ ] 🔮 Historical data (store and display last hours/days)  

---

## 7. Authentication & Security

- [ ] 🔮 Two-factor authentication  

---

## 11. Advanced Networking

- [ ] 🔮 mDNS / Bonjour forwarding (Chromecast, AirPlay across network segments)  
- [ ] 🔮 Custom routing rules  
- [ ] 🔮 VLAN configuration  

---

## 12. UX & UI Polish

- [ ] 🔮 Multi-language support (i18n)  

---

## 13. Deployment & Packaging

- [ ] 🔮 Automatic updates mechanism  

---

## 14. Hardware Buttons

- [ ] 🔮 Custom button action scripting  
- [ ] 🔮 Long-press vs short-press differentiation. See [Hardware Buttons plan](../plans/hardware-buttons.md#phase-4--long-press-vs-short-press-future).  

---

## 16. General 

- [ ] Investigate if it does make sense to introduce a very lightweight database something like SQLite to store passwords for travo and the data we collected like cpu, traffic usage etc. This needs to be thoroughly investigate because of size usage etc if it would make sense and is feasible. 

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
