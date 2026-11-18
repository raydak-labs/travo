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

The UI answers on `http://<router>/` by default, and LuCI moves to ports 8080 and 8443. To leave LuCI in place, pass `--port N` during install and open `http://<router>:N`. Remove it with `sh /tmp/tg/install.sh --uninstall`; add `--purge` to also delete its config and state. Settings live in `/etc/config/travo-greenfield`. Build the tarball yourself with `mise run package`.

The dashboard shows the active uplink, carrier, and internet reachability separately. Its **Connect to WiFi** button opens the WiFi page even when connectivity is unavailable. Its **VPN** card shows the tunnel state and enables or disables the tunnel with the same confirm-or-roll-back flow as the VPN page. It also shows one-minute CPU load, memory use, uptime, and the running version. Connectivity and resource snapshots refresh every five seconds, and interface throughput every two seconds, over one shared live stream. If the stream disconnects, the UI keeps the last known values marked stale while it reconnects and polls every five seconds.

The **WiFi** page summarizes configured radios, uplink STA and access points, and its **Scan** button lists nearby networks without changing live state.

Use **System → Download backup** to save the router configuration as `travo-config.tar.gz`.
Use **System → Restart router** to restart the router after confirming; the page reconnects when it is back.

System can check GitHub for a newer stable release. An automatic attempt occurs only when opening System, at most once per 24 hours. Use **Check for updates** to check manually. Set `travo-greenfield.main.release_source` to an alternate GitHub `OWNER/REPO` or `off`, then restart the daemon. The default is `blackdark-org/travo-greenfield`.

When System finds a newer release, **Verify package** checks the signed archive and stages it for up to 30 minutes. Verification leaves the installed version running.

Use **VPN** to import and select a WireGuard profile, then enable or disable the tunnel. Confirm the change once the router verifies connectivity, or roll it back. The page also shows the current tunnel state and handshake age. **Split tunnel** sends only the IPv4 prefixes you list through the VPN (the profile's DNS servers are added automatically); leave it empty for a full tunnel. The **Kill switch** (enable it while the tunnel is connected, then confirm) blocks LAN devices from reaching the internet outside the tunnel, also while the tunnel is down or off, until you disable it. While the tunnel is connected, the router resolves DNS through the profile's `DNS =` servers; **Verify DNS** checks that and runs a test lookup.

**Services** shows AdGuard Home. It installs once after the first Internet connection, listens on loopback only, and becomes the default DNS once it resolves. Start or stop it, turn "default DNS" on or off, and open its own UI through the login-protected port 3001. If AdGuard Home stops answering, DNS falls back to the uplink resolver and the page says so.

On the WiFi page, scan for an uplink, join it on a free radio, and confirm the connection once it is verified. If the only radio that fits the network carries your access point, the join dialog says so and the access point moves to the other radio in the same change. **Access point settings** edits each access point's name and password and enables or disables it, with the same verify-then-confirm flow. Disabling the access point you are connected through is refused unless you choose **Apply anyway**. **Guest network** sets up a separate access point on 192.168.2.0/24 with internet access only. **Uplink MAC address** switches the uplink between the factory, a random or a custom MAC, or copies the MAC of the device you are using (for hotel networks that registered it).

**Saved uplinks** keeps an ordered list of networks. Use **Save** in a network's dialog to store its credentials without changing the current connection. Move a network up or down to change its priority. **Select** joins a saved network with the same confirm or rollback flow as Join. **Forget** removes a saved network and leaves the current connection up.

When the WiFi uplink drops, the router first lets the WiFi client reconnect on its own. If that fails, it retries the configured network up to 5 times, about two minutes apart. It never switches networks by itself. After the last attempt, the WiFi page says so. If a scan shows a saved network in range, the page offers the highest-priority one to **Select**. The attempt count survives restarts and resets once the uplink has an address again.

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
