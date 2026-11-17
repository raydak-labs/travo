# 0006 AdGuard Home: package, topology, proxy, fail-open

Status: accepted; S1, S2, S5 verified on GL-AXT1800 25.12.6 (testdata/device/axt1800-25.12.6/SPIKES.md)

- **Install source**: OpenWrt `adguardhome` package via the PackageManager port. OpenWrt owns binary and conffile (survives sysupgrade); Travo supervises and configures. Rejected: upstream binary in `/opt` (no conffile tracking, schema downgrade crash loop).
- **Topology**: dnsmasq stays on :53 and forwards to AGH on `127.0.0.1:5353`. dnsmasq remains the single DNS authority for router self-resolution, DHCP names, hotel DNS, VPN DNS and Tailscale (`--accept-dns=false`, resolver as a layer). Cost: AGH sees queries from 127.0.0.1 (no per-client stats). Expert option later: AGH on :53 via the layer stack.
- **Access**: AGH binds loopback, no accounts. Travo serves a gated reverse proxy on `:3001` (own port: no base-path rewriting). Fallback if the proxy fails: sync one AGH user at login/password change.
- **Travo UI**: on/off and "default DNS" toggle only; everything else in AGH's UI.
- **First boot**: no uplink means no package. Install after the first successful uplink as a guarded package op with a flash check; DNS stays at base until the liveness probe passes.
- **Failure**: liveness probe resolves a fresh name via 127.0.0.1:5353; after M failures pop the `adguard` layer (fail open), show a banner, re-enable only on user confirm or after a hold-down of passing probes. Refuse AGH downgrades over a newer config.

Verified facts: package 0.107.79-r1, config `/etc/adguardhome/adguardhome.yaml` (conffile, kept by sysupgrade), runs in ujail. A root-path reverse proxy on its own port works with `users: []`. Required change from the package/old-Travo state: `http.address` must become `127.0.0.1:3000` (it was `0.0.0.0:3000`, admin API open to the LAN) and DNS `bind_hosts` should be `127.0.0.1` only. Migration from an existing install must do both under a transaction.

## Implementation notes (#132)

- **Config migration is a guarded file swap, not a `txn` transaction.** The AGH YAML is not UCI and `txn` snapshots `/etc/config` only. `internal/adguard` keeps the old file in the state dir, writes the new one atomically, restarts AGH and requires `GET 127.0.0.1:3000/control/status` = 200 within 20 s; otherwise it restores the old file and restarts. A failed restore keeps the `adguard-config-in-progress` guard; at startup `Recover` puts a kept file back.
- **No YAML library.** The migration edits only `http.address`, `dns.bind_hosts` and `dns.port` in the two-space block style AGH writes, and refuses any other shape (`adguard_config_unknown`) instead of guessing. This keeps the binary free of a YAML dependency for two keys.
- **Fresh install seeds the config first.** `internal/adguard/seed.yaml` is the captured 0.107.79 config passed through the same migration (a test keeps them equal). It is written before `apk add`, so AGH never starts its first-run wizard on the LAN. One automatic install per router (marker in the state dir), only while the Internet probe is online and the overlay has 52 MiB free (32 MiB installed size + 20 MB headroom); the user's explicit install clears the install guard by name.

## Implementation notes (#133)

- **Adopt before the first pop.** Old Travo wrote `server 127.0.0.1#5353` + `noresolv=1` as the dnsmasq base, so popping the `adguard` layer over that base changed nothing. `dnslayer.Adopt` records an exact live match as the active `adguard` layer over a base without `server`/`noresolv` (dnsmasq then reads its resolv file); it writes no DNS option and refuses when a record already exists.
- **Probe**: a raw UDP query for `travo-<random>.example.com` to `127.0.0.1:5353`, 5 s timeout, every 15 s. NOERROR/NXDOMAIN pass; SERVFAIL, REFUSED and timeouts fail. Ticks are skipped without a kernel default route and while the `captive` layer is active (the portal hides the Internet; captive wins anyway).
- **Fail-open**: 3 consecutive failures pop the layer and persist `fail_open_since` in `/etc/travo-greenfield/adguard-dns.json`. Re-enable: the user's `POST /api/v1/adguard/dns {"enabled":true}` (needs a passing probe) or 10 minutes of continuously passing probes. A user "off" persists there too and is never undone by the probe.
- **Default on**: without a user "off", the first passing probe after install pushes the layer (DNS stays at base until then). Turning AGH off pops the layer before `stop`.

## Implementation notes (#134)

- The proxy listens on the daemon's `--listen` host, port 3001 (`--adguard-proxy-listen` overrides, `off` disables; off by default in mock mode). Gate: LAN source and Host check as for the API, a valid `tg_session` cookie (cookies are not port-scoped, so the Travo login carries over), and for unsafe methods an `Origin` equal to the proxy's own origin instead of the CSRF header. The session cookie is stripped before forwarding; the incoming Host is kept. Unauthenticated requests get 401 JSON (no redirect).
