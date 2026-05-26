---
title: AmneziaWG — DPI-Resistant VPN
description: How AmneziaWG extends WireGuard with obfuscation for censored networks.
updated: 2026-05-26
tags: [vpn, wireguard, amneziawg, dpi, obfuscation, censorship]
---

# AmneziaWG — DPI-Resistant VPN

## What is AmneziaWG?

AmneziaWG is a **WireGuard-compatible protocol** that adds packet obfuscation to defeat Deep Packet Inspection (DPI). It keeps WireGuard's speed and simplicity while making traffic invisible to network censorship systems.

Standard WireGuard has a distinctive packet signature that firewalls in China, Russia, Iran, UAE, and other countries actively detect and block. AmneziaWG solves this by:

- Adding **junk packets** between real data to break pattern recognition
- **Padding** handshake messages to variable sizes
- **Replacing protocol header constants** with random values so packets don't match known WireGuard fingerprints

## Why it matters for travel routers

A travel router's primary job is to provide reliable internet access anywhere in the world. When you connect to hotel or airport WiFi in a country with internet censorship, standard WireGuard VPN connections often get blocked within minutes.

AmneziaWG makes your VPN connection **look like random UDP traffic**, which is much harder for DPI systems to identify and block.

## How it works in Travo

### Automatic detection

When you import a `.conf` file that contains AmneziaWG parameters, Travo automatically detects it as an AWG profile. You'll see a purple **AWG** badge next to the profile name.

### Coexistence with standard WireGuard

- **Standard WireGuard profiles continue to work** exactly as before
- You can have both WireGuard and AmneziaWG profiles saved simultaneously
- Only one VPN tunnel is active at a time (same rule as always)
- Kill switch, split tunnel, DNS leak protection — all work identically with AWG

### Required packages

AmneziaWG requires additional packages not included in standard OpenWrt:

- `amneziawg-tools` — the `awg` userspace tool
- `kmod-amneziawg` — the kernel module

If you import an AWG profile but packages aren't installed, you'll see an install prompt. Install from the **Services** page.

## AmneziaWG parameters explained

AWG config files include extra fields in the `[Interface]` section:

| Parameter | Purpose |
|-----------|---------|
| `Jc` | Junk packet count — how many decoy packets to send |
| `Jmin` | Minimum junk packet size (bytes) |
| `Jmax` | Maximum junk packet size (bytes) |
| `S1` | Init handshake padding size |
| `S2` | Response handshake padding size |
| `H1`–`H4` | Header magic values (replace WireGuard's fixed constants) |

These values must match between client and server — they're part of the profile you get from your VPN provider or self-hosted server.

## Getting an AmneziaWG profile

### From a VPN provider

Some privacy-focused VPN providers offer AmneziaWG configs. Look for providers that mention "anti-censorship" or "obfuscated WireGuard" support.

### Self-hosted

Run an AmneziaWG server using:
- [AmneziaVPN](https://amnezia.org/) — easy self-hosted VPN app
- Manual setup with `amneziawg-tools` on any Linux server

The server generates a `.conf` file with the obfuscation parameters already configured.

## Example config

```ini
[Interface]
PrivateKey = <your-private-key>
Address = 10.8.0.2/32
DNS = 1.1.1.1
Jc = 4
Jmin = 40
Jmax = 70
S1 = 55
S2 = 33
H1 = 1234567890
H2 = 987654321
H3 = 1122334455
H4 = 5566778899

[Peer]
PublicKey = <server-public-key>
AllowedIPs = 0.0.0.0/0
Endpoint = your-server.com:51820
```

## Comparison with alternatives

| Approach | Speed | Ease of use | DPI resistance | Requires server changes |
|----------|-------|-------------|----------------|------------------------|
| **AmneziaWG** | ★★★★★ | ★★★★★ | ★★★★☆ | Yes (AWG server) |
| Standard WireGuard | ★★★★★ | ★★★★★ | ★☆☆☆☆ | No |
| Shadowsocks + WG | ★★★☆☆ | ★★☆☆☆ | ★★★★☆ | Yes (proxy chain) |
| V2Ray/Xray | ★★★☆☆ | ★★☆☆☆ | ★★★★★ | Yes (complex setup) |
| WG over TCP (udp2raw) | ★★☆☆☆ | ★★☆☆☆ | ★★★☆☆ | Yes |

AmneziaWG offers the best balance of speed, simplicity, and DPI resistance for a travel router.

## References

- [AmneziaWG 2.0 announcement](https://amnezia.org/blog/amneziawg-2-0-available-for-self-hosted)
- [AmneziaWG kernel module (GitHub)](https://github.com/amnezia-vpn/amneziawg-linux-kernel-module)
- [AmneziaWG tools (GitHub)](https://github.com/amnezia-vpn/amneziawg-tools)
- [[adr/0008-wireguard-family-protocol-coexistence|ADR 0008: WireGuard-family coexistence]]
- [[plans/amneziawg-integration|Implementation plan]]
