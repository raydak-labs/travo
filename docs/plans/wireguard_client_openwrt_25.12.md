# WireGuard Client Configuration Plan – OpenWrt 25.12

## Overview

This plan configures OpenWrt as a WireGuard **client** that routes all LAN traffic through a remote WireGuard server. Steps use UCI CLI commands to ensure reproducibility and agent compatibility.

> **OpenWrt 25.12 Note:** This release replaced `opkg` with **APK (Alpine Package Keeper)** as the default package manager. All package installation commands below use `apk`.

---

## Prerequisites

- OpenWrt 25.12 installed and running
- SSH/root access to the router
- A working WireGuard **server** already set up with the following info on hand:
  - Server public key (`SERVER_PUB`)
  - Server endpoint IP or FQDN (`SERVER_ENDPOINT`)
  - Server UDP port (`SERVER_PORT`, default `51820`)
  - Preshared key (`PRESHARED_KEY`, optional but recommended)
  - Assigned client tunnel IP/CIDR (`CLIENT_TUNNEL_ADDR`, e.g. `10.0.0.2/24`)
  - DNS server reachable via the tunnel (`TUNNEL_DNS`, e.g. `10.0.0.1`)

---

## Step 1 – Install Required Packages

```sh
# OpenWrt 25.12 uses apk (Alpine Package Keeper) — NOT opkg
apk update
apk add wireguard-tools kmod-wireguard
```

> Note: `kmod-wireguard` may already be built into the kernel in 25.12. Running `apk add` will skip it if already present.

---

## Step 2 – Generate WireGuard Key Pair

```sh
# Generate private key and derive public key
wg genkey | tee /tmp/wg_private.key | wg pubkey > /tmp/wg_public.key

# Display both — save these values
cat /tmp/wg_private.key   # CLIENT_PRIVATE_KEY
cat /tmp/wg_public.key    # CLIENT_PUBLIC_KEY (register this on the server side)
```

> **Important:** Register `CLIENT_PUBLIC_KEY` on the WireGuard server as a peer with the allowed IP matching `CLIENT_TUNNEL_ADDR`.

---

## Step 3 – Define Variables

Set all values in shell variables for reuse in subsequent commands:

```sh
WG_IF="wg0"
CLIENT_PRIVATE_KEY="<paste CLIENT_PRIVATE_KEY here>"
CLIENT_TUNNEL_ADDR="10.0.0.2/24"       # Assigned tunnel IP from server
SERVER_PUB="<paste SERVER_PUB here>"
PRESHARED_KEY="<paste PRESHARED_KEY here>"    # Leave empty string if not used
SERVER_ENDPOINT="vpn.example.com"      # or IP address
SERVER_PORT="51820"
TUNNEL_DNS="10.0.0.1"                  # DNS via tunnel to prevent DNS leaks
```

---

## Step 4 – Configure the WireGuard Network Interface

```sh
# Remove any existing config for this interface
uci -q delete network.${WG_IF}

# Create WireGuard interface
uci set network.${WG_IF}="interface"
uci set network.${WG_IF}.proto="wireguard"
uci set network.${WG_IF}.private_key="${CLIENT_PRIVATE_KEY}"
uci add_list network.${WG_IF}.addresses="${CLIENT_TUNNEL_ADDR}"

# Commit network config
uci commit network
```

---

## Step 5 – Configure the WireGuard Peer (Server)

```sh
# Remove any existing peer config
uci -q delete network.wgserver

# Define peer (the remote WireGuard server)
uci set network.wgserver="wireguard_${WG_IF}"
uci set network.wgserver.public_key="${SERVER_PUB}"
uci set network.wgserver.preshared_key="${PRESHARED_KEY}"
uci set network.wgserver.endpoint_host="${SERVER_ENDPOINT}"
uci set network.wgserver.endpoint_port="${SERVER_PORT}"
uci set network.wgserver.persistent_keepalive="25"
uci set network.wgserver.route_allowed_ips="1"
uci add_list network.wgserver.allowed_ips="0.0.0.0/0"
uci add_list network.wgserver.allowed_ips="::/0"    # Include IPv6 if needed

uci commit network
```

> `route_allowed_ips=1` instructs OpenWrt to automatically create routes for all allowed IPs through this tunnel.
> `persistent_keepalive=25` keeps the tunnel alive through NAT.

---

## Step 6 – Configure the Firewall

### 6a – Create a WireGuard Firewall Zone

```sh
# Add a new firewall zone for WireGuard
uci add firewall zone
uci set firewall.@zone[-1].name="${WG_IF}"
uci set firewall.@zone[-1].input="DROP"
uci set firewall.@zone[-1].output="ACCEPT"
uci set firewall.@zone[-1].forward="DROP"
uci set firewall.@zone[-1].masq="1"
uci set firewall.@zone[-1].mtu_fix="1"
uci add_list firewall.@zone[-1].network="${WG_IF}"
```

### 6b – Allow LAN Traffic to Forward Through the WireGuard Zone

```sh
# Add lan → wg0 forwarding
uci add firewall forwarding
uci set firewall.@forwarding[-1].src="lan"
uci set firewall.@forwarding[-1].dest="${WG_IF}"
```

> **Note:** If you want WireGuard to be the *only* egress for LAN clients, remove or disable the default `lan → wan` forwarding rule. If split-tunnel is desired, keep both rules and adjust `allowed_ips` on the peer to specific subnets instead of `0.0.0.0/0`.

### 6c – Allow the WireGuard UDP Port Through the WAN

```sh
uci add firewall rule
uci set firewall.@rule[-1].name="Allow-WireGuard-Out"
uci set firewall.@rule[-1].src="wan"
uci set firewall.@rule[-1].dest_port="${SERVER_PORT}"
uci set firewall.@rule[-1].proto="udp"
uci set firewall.@rule[-1].target="ACCEPT"
uci set firewall.@rule[-1].direction="out"

uci commit firewall
service firewall restart
```

---

## Step 7 – DNS Configuration (Prevent DNS Leaks)

Route DNS queries through the WireGuard tunnel to the server-side DNS resolver:

```sh
# Point dnsmasq at the tunnel DNS server
uci add_list dhcp.@dnsmasq[0].server="${TUNNEL_DNS}"

# Optional: disable dnsmasq from using upstream resolv.conf (forces only tunnel DNS)
uci set dhcp.@dnsmasq[0].noresolv="1"

uci commit dhcp
service dnsmasq restart
```

> If the VPN server uses a different DNS (e.g., `1.1.1.1` or `9.9.9.9`) instead of an internal tunnel IP, replace `TUNNEL_DNS` accordingly.

---

## Step 8 – Apply Network Changes

```sh
service network restart
```

Wait ~10 seconds for the interface to come up, then proceed to verification.

---

## Step 9 – Verify the Connection

```sh
# Check WireGuard interface status
wg show

# Verify the tunnel IP is assigned
ip addr show ${WG_IF}

# Check routing table for 0.0.0.0/0 via wg0
ip route show table main | grep ${WG_IF}

# Ping through the tunnel (use server's tunnel IP)
ping -I ${WG_IF} -c 4 ${TUNNEL_DNS}

# Confirm public IP is now the VPN server's IP
curl -s https://ifconfig.me
```

---

## Step 10 – Persist Keys Securely (Optional but Recommended)

```sh
# Move keys out of /tmp to a persistent location
mkdir -p /etc/wireguard
cp /tmp/wg_private.key /etc/wireguard/private.key
chmod 600 /etc/wireguard/private.key

# Clean up temp files
rm /tmp/wg_private.key /tmp/wg_public.key
```

---

## Step 11 – Enable Auto-Start on Boot

WireGuard interfaces managed via UCI/netifd start automatically on boot.
Verify the network service is enabled:

```sh
service network enable
```

---

## Checklist

- [ ] `wireguard-tools` and `kmod-wireguard` installed via `apk add`
- [ ] Client key pair generated; public key registered on server
- [ ] WireGuard interface (`wg0`) configured with private key and tunnel address
- [ ] Peer configured with server public key, endpoint, keepalive, and `allowed_ips`
- [ ] Firewall zone `wg0` created with masquerading enabled
- [ ] `lan → wg0` forwarding rule added
- [ ] WAN rule allows outbound UDP on `SERVER_PORT`
- [ ] DNS pointing to tunnel-side resolver (no DNS leak)
- [ ] `wg show` confirms handshake with server
- [ ] Public IP verified via tunnel
- [ ] Private key stored securely with `chmod 600`

---

## Notes for Agent

- Replace all `<placeholder>` values before execution.
- If `PRESHARED_KEY` is not used, omit the `uci set network.wgserver.preshared_key` line entirely.
- For **split-tunnel** (only specific subnets through VPN): change `allowed_ips` from `0.0.0.0/0` to target subnets and keep the `lan → wan` forwarding rule active alongside `lan → wg0`.
- For **IPv6**: add `::/0` to `allowed_ips` and assign an IPv6 tunnel address via `uci add_list network.${WG_IF}.addresses`.
- If the WAN interface uses PPPoE or is non-standard, ensure the WireGuard UDP traffic egresses the correct WAN device.
- OpenWrt 25.12 uses **nftables** for firewall (fw4). All `uci firewall` commands are compatible with fw4.
- OpenWrt 25.12 uses **apk** (Alpine Package Keeper). Do NOT use `opkg` — it is no longer available.
  - `opkg update && opkg install` → `apk update && apk add`
  - `opkg remove` → `apk del`
  - `opkg list-installed` → `apk list --installed`
