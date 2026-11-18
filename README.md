# travo-greenfield

**Travel-router web UI and API for OpenWrt (GL.iNet Slate AX, Beryl AX).** Join hotel WiFi, handle captive portals, see if you are online, switch VPN on and off, with DNS filtering by AdGuard Home. Runs on the router next to LuCI.

Status: design phase, no code yet. Target 0.1.0.

## Install

On the router, as root (OpenWrt 25.12 or newer, aarch64). Download `travo-greenfield-<version>-aarch64.tar.gz` and `checksums.txt.sig` from the GitHub release and copy both over. Extract, put the `.sig` next to the extracted files, then run the installer; it verifies the signature with the router's `usign` before it touches anything:

```sh
mkdir -p /tmp/tg && tar -xzf travo-greenfield-<version>-aarch64.tar.gz -C /tmp/tg
cp checksums.txt.sig /tmp/tg/
sh /tmp/tg/install.sh
```

Start with a fresh OpenWrt installation. The first install trusts the release key inside the tarball and stores it in `/etc/travo-greenfield/minisign.pub`; later installs must match that key, or pass `--rotate-key` once when the release notes announce a new one. Builds you made yourself are unsigned: install them with `--insecure`.

The UI answers on `http://<router>/` by default, and LuCI moves to ports 8080 and 8443. `https://<router>/` serves the same UI with a self-signed certificate generated on the router (accept the browser warning once; the fingerprint is in `logread -e travo-greenfield`). Set `uci set travo-greenfield.main.listen_tls=off` to turn HTTPS off. To leave LuCI in place, pass `--port N` during install and open `http://<router>:N`. Remove it with `sh /tmp/tg/install.sh --uninstall`; add `--purge` to also delete its config and state. Settings live in `/etc/config/travo-greenfield`. Build the tarball yourself with `mise run package`.

The dashboard shows the active uplink, carrier, and internet reachability separately. Its **Connect to WiFi** button opens the WiFi page even when connectivity is unavailable. Its **VPN** card shows the tunnel state and enables or disables the tunnel with the same confirm-or-roll-back flow as the VPN page. Behind a hotel portal, the **Hotel WiFi login** card opens the login page, can **Try automatic login** for click-through portals (it ticks the terms box and connects, and stops when the page asks for your details such as a room number), switches to the hotel DNS for 5 minutes, and renews the WiFi lease. It also shows one-minute CPU load, memory use, uptime, and the running version. Connectivity and resource snapshots refresh every five seconds, and interface throughput every two seconds, over one shared live stream. If the stream disconnects, the UI keeps the last known values marked stale while it reconnects and polls every five seconds.

The **WiFi** page summarizes configured radios, uplink STA and access points, and its **Scan** button lists nearby networks without changing live state.

**System → Device settings** changes the router's hostname, time zone and NTP time servers.

**System → SSH keys** lists, adds and removes the public keys that may log in to the router as root (dropbear `authorized_keys`); lines it does not manage stay untouched.
**System → System log** shows the router's log (newest first) with a level and text filter.

**Alerts**: a banner above every page names conditions that need you (a guard left behind by an interrupted operation, settings kept in memory only, AdGuard Home bypassed after it stopped answering) and links to the page that fixes them; `GET /api/v1/system/alerts` also lists warnings such as low flash space or an exhausted uplink reconnect.

Use **System → Download backup** to save the router configuration as `travo-config.tar.gz`.
Use **System → Restart router** to restart the router after confirming; the page reconnects when it is back.

System can check GitHub for a newer stable release. An automatic attempt occurs only when opening System, at most once per 24 hours. Use **Check for updates** to check manually. Set `travo-greenfield.main.release_source` to an alternate GitHub `OWNER/REPO` or `off`, then restart the daemon. The default is `blackdark-org/travo-greenfield`.

When System finds a newer release, **Verify package** checks the signed archive and stages it for up to 30 minutes. Verification leaves the installed version running.

Use **VPN** to import and select a WireGuard profile, then enable or disable the tunnel. Risky actions (disabling the tunnel, the kill switch, deleting a profile) ask in a dialog first. While a change waits for confirmation, a notification counts down the rollback window with **Keep change** and **Roll back**; results (confirmed, rolled back, failed) appear as notifications too. The page also shows the current tunnel state and handshake age. **Split tunnel** sends only the IPv4 prefixes you list through the VPN (the profile's DNS servers are added automatically); leave it empty for a full tunnel. The **Kill switch** (enable it while the tunnel is connected, then confirm) blocks LAN devices from reaching the internet outside the tunnel, also while the tunnel is down or off, until you disable it. While the tunnel is connected, the router resolves DNS through the profile's `DNS =` servers; **Verify DNS** checks that and runs a test lookup.

**VPN → Tailscale** installs the Tailscale package on request (it needs Internet and about 46 MB of flash), shows a login link to join your tailnet, and lists the router's tailnet name and addresses. Pick an exit node (refused while the WireGuard tunnel is on; it falls back if the exit node has no Internet), share the LAN with the tailnet, or turn on Tailscale SSH. LAN clients reach the tailnet after you apply and confirm the Tailscale firewall change. Tailnet names resolve for LAN clients through the router's DNS; Tailscale never takes over the router's DNS. The dashboard shows the Tailscale state once it is installed.

**Services** shows AdGuard Home. It installs once after the first Internet connection (or right away without one: download the release's `adguardhome-*.apk` too and pass it as `install.sh --adguard-bundle <file>`), listens on loopback only, and becomes the default DNS once it resolves. Start or stop it, turn "default DNS" on or off, and open its own UI through the login-protected port 3001. If AdGuard Home stops answering, DNS falls back to the uplink resolver and the page says so.

On the WiFi page, scan for an uplink, join it on a free radio, and confirm the connection once it is verified. If the only radio that fits the network carries your access point, the join dialog says so and the access point moves to the other radio in the same change. **Access point settings** edits each access point's name and password and enables or disables it, with the same verify-then-confirm flow. Disabling an access point asks first; disabling the one you are connected through is refused, and a dialog offers **Apply anyway**. **Guest network** sets up a separate access point on 192.168.2.0/24 with internet access only. **Uplink MAC address** switches the uplink between the factory, a random or a custom MAC, or copies the MAC of the device you are using (for hotel networks that registered it).

**Saved uplinks** keeps an ordered list of networks. Use **Save** in a network's dialog to store its credentials without changing the current connection. Move a network up or down to change its priority. **Select** joins a saved network with the same confirm or rollback flow as Join. **Forget** removes a saved network and leaves the current connection up.

When the WiFi uplink drops, the router first lets the WiFi client reconnect on its own. If that fails, it retries the configured network up to 5 times, about two minutes apart. It never switches networks by itself. After the last attempt, the WiFi page says so. If a scan shows a saved network in range, the page offers the highest-priority one to **Select**. The attempt count survives restarts and resets once the uplink has an address again.

On the Network page, **USB tethering** uses a phone as an uplink. Install the Android or iPhone drivers there, connect the phone by USB with USB tethering (Android) or Personal Hotspot (iPhone) on, and choose **Use as uplink**: the phone joins the wan zone with the usual confirm or rollback flow and carries traffic while Ethernet and WiFi have no connection. The dashboard names the medium of the active uplink (Ethernet, WiFi, USB tethering). **Failover** orders two or three uplinks (Ethernet, WiFi, USB tethering): IPv4 traffic uses the first one that is up and moves to the next when it drops. Enabling installs mwan3, verifies the change with a real connection test and waits for your confirmation; IPv6 keeps the normal route.

**Clients** (`#/clients`) lists the devices on your network with their address, MAC and how they connect (WiFi with signal, LAN, or offline). **Edit** gives a device a name and can reserve its current address (it gets it on its next lease renewal). **Block** cuts a device's internet access with the same confirm or rollback flow as WiFi changes; it can still open the admin page. **Disconnect** drops a WiFi device once; it may rejoin. Your own device can be neither blocked nor disconnected.

## Automation

Scripts on the LAN use an API token instead of the browser login: create, list and revoke them under System → Manage API tokens (`#/system/tokens`), or create one with `POST /api/v1/auth/tokens` (`{"name":"ci","scope":"read"}` or `"admin"`) from a logged-in session, then send it as a bearer header. The token is shown once; revoke it with `DELETE /api/v1/auth/tokens/{id}`. `read` tokens only GET; no token can manage tokens or change the password ([ADR 0008](docs/adr/0008-auth-and-trust-boundary.md)).

```sh
curl -H "Authorization: Bearer tgt_..." http://192.168.8.1/api/v1/system/info
```

## UI components

The Svelte UI uses locally owned shadcn-svelte components, with the same light/dark palette and offline assets. New controls compose these primitives through the existing app wrappers. See [ADR 0013](docs/adr/0013-shadcn-components.md) for the size tradeoffs and [migration evidence](docs/testing/shadcn-migration.md) for the inventory and keyboard checks.

## Docs

- [Spec](docs/spec.md) · [Invariants](docs/invariants.md) · Plan: [milestone 0.1.0](https://github.com/blackdark-org/travo-greenfield/milestone/1) and [project board](https://github.com/orgs/blackdark-org/projects/1)
- [Device facts](docs/device-facts.md) · [Testing](docs/testing.md) · [Glossary](GLOSSARY.md) · [ADRs](docs/adr/)

License: [MIT](LICENSE).

On a packaged OpenWrt install, use **System → Software updates** to verify an available release and confirm **Apply update**. The interface briefly disconnects while the service restarts. Sign in again to see the result. If the updater reports that recovery is required, use **Retry recovery**, or use the console command documented in ADR 0007.
