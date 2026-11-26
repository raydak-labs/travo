# 0034 First install: LAN off 192.168.1.0/24, WiFi set up once

Status: accepted

Amends the apply rule of [invariants](../invariants.md) §1 ("apply paths write UCI only") for
`install.sh`, and [0007](0007-distribution-and-self-update.md) (what the installer changes).

## Context

A travel router is reached over WiFi, but OpenWrt ships both access points off and open
(`testdata/device/axt1800-25.12.6/factory-default/`). After a fresh install nobody could reach
Travo without a cable, and nothing told them so.

OpenWrt's factory LAN is `192.168.1.1/24`. Hotel, home and phone-hotspot networks often use
the same subnet; an uplink inside the LAN's subnet breaks routing. The guarded LAN change
([0028](0028-lan-address-change.md)) fixes it later, but only once someone notices.

## Decision

- **WiFi in the UI.** `POST /api/v1/wifi/setup` gives every AP bridged into `lan` one name
  (default `Travo`) and key, `sae-mixed` (WPA2/WPA3), and enables it, in one guarded
  transaction like an AP edit: the probe proves each AP broadcasting, then the user confirms.
  One name on both bands, because an uplink join may move an AP to the other radio
  (invariants §10); with one name and key clients follow without noticing. APs on a radio that
  carries an enabled STA stay off (ath11k, device-facts §3).
- **Alert `wifi_not_set_up`** (danger, banner on every page) while every main AP is off and
  open, as shipped. An AP a schedule switched off keeps its key, so a WiFi that is merely off
  does not raise it. The banner links to the WiFi page, whose first card sets it up.
- **Installer, first install only** (no init script before). All three steps replace only OpenWrt's
  factory values; anything an operator changed stays:
  - LAN `192.168.1.1/24` becomes `192.168.50.1/24` (`--lan CIDR` picks another, `--keep-lan`
    skips). Skipped when an uplink already holds an address in the target /24.
  - Hostname `OpenWrt` (`system.@system[0].hostname`) becomes `travo` (`--hostname NAME` picks
    another RFC 1123 label, `--keep-hostname` skips). When changed, `travo.main.host_allow`
    updates from `openwrt.lan` to `<hostname>.lan` so the DNS-rebinding guard accepts it.
  - With `--wifi` (asks on the terminal) or `TRAVO_WIFI_PASSWORD`, factory APs get the same
    settings as the UI setup (`--wifi-name`, default `Travo`). The key reaches `uci batch`
    through stdin, never argv. The password is checked before anything is installed.
  - UCI is committed after the install is healthy and LuCI has moved. When LAN or WiFi changed,
    `reload_config` then runs detached after the installer exits, since it drops an SSH session on
    the LAN, followed by one restart of Travo so its self-signed certificate and host allowlist
    match the new address and hostname. When only the hostname changed, `reload_config` and the
    Travo restart run synchronously without dropping SSH. No `wifi` command, as everywhere else.

## Consequences

- The installer changes live network state without a confirm window. Accepted: it runs only
  on a first install, only over factory values, from a root shell whose operator chose the
  options, and the result is printed (new address, hostname, WiFi name). The device that ran it
  keeps its old lease until it reconnects; the installer says so.
- The UI path stays fully guarded; only the installer is exempt.
- `mise run test:package` checks the LAN move, the factory hostname change and custom-hostname
  preservation, the staged WiFi on both radios, the quoted key and that nothing is left
  uncommitted. The live AP and the reload are hardware-only (`docs/testing.md`, hardware playbook).
