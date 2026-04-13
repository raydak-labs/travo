---
title: "Plan: WireGuard Full Networking Setup"
description: "Planning / design notes: Plan: WireGuard Full Networking Setup"
updated: 2026-04-13
tags: [plan, traceability, wireguard]
---

# Plan: WireGuard Full Networking Setup

**Status:** Not implemented
**Priority:** High
**Related requirements:** [3.1 WireGuard](../requirements/tasks_done.md#vpn-and-services)
**Reference:** [WireGuard Client OpenWRT 25.12 (CLI steps)](wireguard_client_openwrt_25.12.md)

---

## Goal

Make WireGuard VPN actually work end-to-end: proper network interfaces, firewall zones, forwarding rules, DNS leak prevention, and a "Verify" button so users can validate their setup. Currently, profile import/edit works but the underlying OpenWRT plumbing (interface, zone, forwarding) is not created automatically.

---

## Scope

1. Auto-install `luci-proto-wireguard` alongside `wireguard-tools` + `kmod-wireguard`
2. When a WireGuard profile is activated, create the full UCI plumbing:
   - `network.wg0` interface (proto wireguard, private key, addresses)
   - `network.wg0_peer0` peer section (public key, endpoint, allowed IPs, keepalive)
   - `firewall` zone `wg0` (masq, mtu_fix)
   - `firewall` forwarding `lan → wg0`
   - DNS forwarding through tunnel (optional, for kill-switch / leak prevention)
3. "Verify VPN" button that checks: interface up, handshake recent, route exists, public IP changed
4. Kill switch improvements (block non-VPN traffic when enabled)
5. Split tunneling (route only selected traffic through VPN)

---

## Phases

### Phase 1 — Package Installation

**When:** User installs WireGuard from Services page
**What:** Install `wireguard-tools`, `kmod-wireguard`, and `luci-proto-wireguard` together.

- Backend: Update service definition to include all 3 packages
- Already partially exists; just add `luci-proto-wireguard` to the install list

**Files:** `backend/internal/services/service_registry.go` (or wherever wireguard service is defined)

### Phase 2 — Profile Activation Creates Full UCI Plumbing

**When:** User activates a WireGuard profile (toggle enable)
**What:** The backend creates all necessary UCI config.

**Steps (following [reference plan](wireguard_client_openwrt_25.12.md)):**

1. **Network interface:**
   ```
   uci set network.wg0=interface
   uci set network.wg0.proto=wireguard
   uci set network.wg0.private_key=<from profile>
   uci add_list network.wg0.addresses=<from profile>
   ```

2. **Peer section:**
   ```
   uci set network.wg0_peer0=wireguard_wg0
   uci set network.wg0_peer0.public_key=<from profile>
   uci set network.wg0_peer0.preshared_key=<from profile, optional>
   uci set network.wg0_peer0.endpoint_host=<endpoint>
   uci set network.wg0_peer0.endpoint_port=<port>
   uci set network.wg0_peer0.persistent_keepalive=25
   uci set network.wg0_peer0.route_allowed_ips=1
   uci add_list network.wg0_peer0.allowed_ips=0.0.0.0/0
   uci add_list network.wg0_peer0.allowed_ips=::/0
   ```

3. **Firewall zone:**
   ```
   uci add firewall zone → name=wg0, input=DROP, output=ACCEPT, forward=DROP, masq=1, mtu_fix=1, network=wg0
   ```

4. **Firewall forwarding:**
   ```
   uci add firewall forwarding → src=lan, dest=wg0
   ```

5. **Commit + apply:**
   ```
   uci commit network
   uci commit firewall
   service network restart
   service firewall restart
   ```

**Important:** When deactivating, the plumbing should be cleaned up:
- Delete `network.wg0`, `network.wg0_peer0`
- Delete the firewall zone and forwarding rule
- Commit and restart

**Files:**
- `backend/internal/services/vpn_service.go` — new methods `setupWireGuardPlumbing()`, `teardownWireGuardPlumbing()`
- `backend/internal/services/vpn_service_test.go` — tests with mock UCI
- Need UCI operations: `AddSection`, `Set`, `AddList`, `DeleteSection`, `Commit`

### Phase 3 — Verify VPN Button

**API:** `GET /api/v1/vpn/wireguard/verify`

**Checks:**
1. `wg0` interface exists and is up (`ip link show wg0`)
2. Latest handshake < 3 minutes ago (`wg show wg0 dump`)
3. Default route through wg0 exists (`ip route show default`)
4. Public IP check — make HTTP request to `https://api.ipify.org` and compare with WAN IP
5. DNS leak check — resolve a known domain and verify response comes from tunnel DNS

**Response:**
```json
{
  "interface_up": true,
  "handshake_ok": true,
  "route_ok": true,
  "public_ip": "198.51.100.1",
  "wan_ip": "203.0.113.5",
  "ip_changed": true,
  "dns_leak": false
}
```

**Files:**
- `backend/internal/services/vpn_service.go` — `VerifyWireGuard()`
- `backend/internal/api/vpn_handlers.go` — `VerifyWireguardHandler`
- `frontend/src/pages/vpn/vpn-page.tsx` — "Verify VPN" button + result display

### Phase 4 — Kill Switch

**When:** User enables "Kill Switch" toggle
**What:** Block all non-VPN LAN→WAN traffic.

**Implementation:**
- Remove or disable `lan → wan` firewall forwarding when kill switch is on
- Re-enable when kill switch is off
- Store kill switch state in `/etc/openwrt-travel-gui/vpn_config.json`

**Files:**
- `backend/internal/services/vpn_service.go` — `SetKillSwitch()`
- Firewall manipulation via UCI

### Phase 5 — Split Tunneling (Future)

**When:** User wants only certain traffic through VPN
**What:** Instead of `allowed_ips=0.0.0.0/0`, set specific subnets.

**Implementation:**
- UI: list of "VPN routes" — e.g., "Route all traffic", "Route only 10.0.0.0/8"
- Backend: modify `allowed_ips` on the peer section
- Advanced: policy-based routing per client IP (requires `ip rule` + `ip route` tables)

This is the most complex part and should be deferred unless needed.

---

## Testing Strategy

- **Unit tests:** Mock UCI operations to verify correct section creation/deletion
- **Integration test on device:** Run the full flow via SSH, check `uci show network`, `uci show firewall`, `wg show`
- **Verify endpoint:** Test against real VPN server

---

## Risks & Notes

- **Crash guard:** The `service network restart` after UCI changes should use the apply/confirm rollback pattern if possible, though network restart (not wifi) is generally safe on ath11k
- **Multiple profiles:** When switching profiles, tear down old plumbing first, then set up new
- **Firewall zone naming:** Use deterministic names (`wg0`, `wg0_zone`, `wg0_fwd`) to find and clean up later
- **luci-proto-wireguard:** Without this package, LuCI won't show the WireGuard interface — important if user wants to debug via LuCI
