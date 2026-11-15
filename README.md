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

The dashboard shows the active uplink, carrier, and internet reachability separately. It also shows one-minute CPU load, memory use, uptime, and the running version. Connectivity and resource snapshots refresh every five seconds, and interface throughput every two seconds, over one shared live stream. If the stream disconnects, the UI keeps the last known values marked stale while it reconnects and polls every five seconds.

The **WiFi** page summarizes configured radios, uplink STA and access points, and its **Scan** button lists nearby networks without changing live state.

Use **System → Download backup** to save the router configuration as `travo-config.tar.gz`.
Use **System → Restart router** to restart the router after confirming; the page reconnects when it is back.

System can check GitHub for a newer stable release. An automatic attempt occurs only when opening System, at most once per 24 hours. Use **Check for updates** to check manually. Set `travo-greenfield.main.release_source` to an alternate GitHub `OWNER/REPO` or `off`, then restart the daemon. The default is `blackdark-org/travo-greenfield`.

When System finds a newer release, **Verify package** checks the signed archive and stages it for up to 30 minutes. Verification leaves the installed version running.

Use **VPN** to import and select a WireGuard profile, then enable or disable the tunnel. Confirm the change once the router verifies connectivity, or roll it back. The page also shows the current tunnel state and handshake age.

On the WiFi page, scan for an uplink, join it on a free radio, and confirm the connection once it is verified.

## Docs

- [Spec](docs/spec.md) · [Invariants](docs/invariants.md) · Plan: [milestone 0.1.0](https://github.com/blackdark-org/travo-greenfield/milestone/1) and [project board](https://github.com/orgs/blackdark-org/projects/1)
- [Device facts](docs/device-facts.md) · [Testing](docs/testing.md) · [Glossary](GLOSSARY.md) · [ADRs](docs/adr/)

License: [MIT](LICENSE).

On a packaged OpenWrt install, use **System → Software updates** to verify an available release and confirm **Apply update**. The interface briefly disconnects while the service restarts. Sign in again to see the result. If the updater reports that recovery is required, use **Retry recovery**, or use the console command documented in ADR 0007.
