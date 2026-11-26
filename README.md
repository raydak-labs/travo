# travo-greenfield

Travel-router web UI and API for OpenWrt (GL.iNet Slate AX, Beryl AX), for travellers who are not network admins and for power users who want an API. Join hotel WiFi, get through captive portals, see whether you are online, switch the VPN on and off, and filter DNS with AdGuard Home. Runs on the router next to LuCI.

## Requirements

- OpenWrt 25.12 or newer on an aarch64 router, starting from a fresh installation
- Root shell on the router (SSH or console)

## Install

Download `travo-greenfield-<version>-aarch64.tar.gz` and `checksums.txt.sig` from the [latest release](https://github.com/blackdark-org/travo-greenfield/releases/latest), copy both to the router, then run as root:

```sh
mkdir -p /tmp/tg && tar -xzf travo-greenfield-<version>-aarch64.tar.gz -C /tmp/tg
cp checksums.txt.sig /tmp/tg/
sh /tmp/tg/install.sh
```

The installer checks the signature with the router's `usign` before it changes anything. The first install trusts the release key in the tarball and stores it in `/etc/travo-greenfield/minisign.pub`. Later installs must match that key; pass `--rotate-key` once when the release notes announce a new one.

| Option | Effect |
|---|---|
| `--port N` | Serve on `http://<router>:N` and leave LuCI on port 80 |
| `--adguard-bundle <file>` | Install AdGuard Home offline from the release's `adguardhome-*.apk` |
| `--insecure` | Install an unsigned build you made yourself |
| `--uninstall` | Remove Travo; add `--purge` to delete its config and state too |

## First use

Open `http://<router>/` and set the password; LuCI moves to ports 8080 and 8443. After `--port N`, open `http://<router>:N/` instead; LuCI stays on port 80. `https://<router>/` serves the same UI with a certificate the router generated itself; its fingerprint is in `logread -e travo-greenfield`. Settings live in `/etc/config/travo-greenfield` (to turn HTTPS off: `uci set travo-greenfield.main.listen_tls=off && uci commit travo-greenfield && /etc/init.d/travo-greenfield restart`).

Changes that can cut you off (WiFi, VPN, firewall, LAN address, WAN) are applied, verified and then wait for you to confirm. If you don't confirm in time, the router rolls the change back.

## Features

| Page | What it does |
|---|---|
| Dashboard | Network map of uplinks, router and clients with live rates and Internet/VPN/Tailscale state, details (WAN protocol, gateway, WiFi band and signal) on hover; uplink cards on phones; hotel WiFi login with automatic click-through; CPU, memory, uptime and version tiles; VPN toggle; per-interface traffic; 24 h / 7 day history |
| Internet | Scan and join WiFi uplinks, saved uplinks by priority, capped automatic reconnect, uplink MAC address; Ethernet WAN (DHCP, static, PPPoE); USB tethering; failover across two or three uplinks; data usage with a monthly budget |
| WiFi | The networks the router offers: access point and guest network settings, on/off schedule |
| VPN → WireGuard, Tailscale | Overview with each VPN and the traffic rules (kill switch, split tunnel, DNS check); WireGuard profiles (add by paste or file) and tunnel switch; Tailscale with exit node, LAN sharing and SSH |
| Clients | Connected devices; name, reserve an address, block, disconnect, wake |
| Services → AdGuard Home | Overview of installed and available services; AdGuard Home (default DNS, DNS over HTTPS, expert topology on port 53); optional SQM, Cloudflare Tunnel, vnStat, Watchcat, speedtestcpp |
| Network → LAN, Port forwards, IPv6 & DDNS, Diagnostics | LAN address, DHCP server and local DNS names; port forwards; IPv6 and dynamic DNS; ping/traceroute/DNS lookup, Wake-on-LAN, interfaces and firewall zones (read-only) |
| System → Hardware, Logs, Firmware, Access | Overview with router facts and the Travo version, device settings and backup; each LED on or off, night mode and the slide switch; system log; Travo updates, OpenWrt release check, firmware upgrade and factory reset; API tokens, SSH keys and the admin password. Restart and Log out are in the header menu |

A banner above every page names conditions that need you, such as a guard left by an interrupted operation, AdGuard Home bypassed, or low memory, and links to the page that fixes them.

## Updating

Under System, Travo checks GitHub for a newer stable release at most once a day, or on **Check for updates**. **Verify package** checks the signed archive and stages it; **Apply update** installs it and restarts the service, so sign in again afterwards. If the updater reports that recovery is required, use **Retry recovery** or the console command in [ADR 0007](docs/adr/0007-distribution-and-self-update.md). To use another release source, set `travo-greenfield.main.release_source` to `OWNER/REPO` (or `off`) and restart the daemon. The Updates page says when checks are turned off or the running build is not a release.

Firmware: **System → Firmware → OpenWrt updates** checks downloads.openwrt.org and links LuCI attended sysupgrade. **System → Firmware → Firmware upgrade** flashes an image you uploaded, after checking that it fits this board.

## Automation

Scripts on the LAN use an API token instead of the browser login. Create one under System → Access, or with `POST /api/v1/auth/tokens` (`{"name":"ci","scope":"read"}` or `"admin"`) from a signed-in session. The token is shown once. `read` tokens can only GET, and no token can manage tokens, change the password or flash firmware ([ADR 0008](docs/adr/0008-auth-and-trust-boundary.md)). The OpenAPI spec is served at `/api/openapi.json`.

```sh
curl -H "Authorization: Bearer tgt_..." http://192.168.8.1/api/v1/system/info
```

## Known issues

- GL-AXT1800 on OpenWrt 25.12.6: the kernel's available memory drifts down over hours while Travo and AdGuard Home stay flat ([device facts](docs/device-facts.md)). The dashboard's System card warns below 15 MiB and the banner shows below 8 MiB; a reboot frees it.

## Development

Needs [mise](https://mise.jdx.dev). No router required: the daemon runs against a mock device scripted from captured fixtures.

```sh
mise run dev          # UI on Vite :18092 against the mock daemon :18081
mise run check        # Go, web, generated client and size checks; must pass before every commit
mise run gen          # regenerate the OpenAPI spec and TypeScript client after API changes
mise run screenshots  # PNGs of every page into web/screenshots/
mise run package      # release tarball
```

Contributors and agents start with [AGENTS.md](AGENTS.md). Work is tracked in [GitHub issues](https://github.com/blackdark-org/travo-greenfield/issues) and on the [project board](https://github.com/orgs/blackdark-org/projects/1).

## Docs

- [Invariants](docs/invariants.md): rules for live state, auth, DNS and WiFi
- [Device facts](docs/device-facts.md): hardware behaviour learned on real routers
- [Testing](docs/testing.md): test layers and the hardware playbook
- [Web UI](docs/web-ui.md): page rules, dialogs and toasts
- [ADRs](docs/adr/README.md) and [glossary](GLOSSARY.md)

License: [MIT](LICENSE).
