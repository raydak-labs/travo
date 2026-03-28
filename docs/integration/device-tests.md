# On-device integration checks

These scripts exercise the **live router** (default `192.168.1.1`). They use the HTTP API and optional SSH. Use the same password as the travel-gui login (often `admin` on lab images).

## Scripts (from repo root)

| Script | Purpose |
|--------|---------|
| `scripts/integration-vpn-wireguard-toggle.sh` | WireGuard enable → disable; router must keep internet (`wget` connectivity check). |
| `scripts/integration-vpn-dns-killswitch.sh` | DNS leak endpoint + kill switch GET; optional `--ssh-verify`, `--enable-killswitch`. |
| `scripts/integration-device.sh` | Broader smoke test (health, WiFi connect/disconnect); needs `test/integration/.wifi_pass`. |

### DNS leak + kill switch (quick)

```sh
./scripts/integration-vpn-dns-killswitch.sh
```

With UCI snapshot over SSH:

```sh
./scripts/integration-vpn-dns-killswitch.sh --ssh-verify
```

Briefly enable kill switch, confirm API and UCI, then disable:

```sh
./scripts/integration-vpn-dns-killswitch.sh --enable-killswitch --ssh-verify
```

### WireGuard toggle (regression)

```sh
./scripts/integration-vpn-wireguard-toggle.sh --login-password 'your-password'
```

## Deploying a new backend before testing

```sh
./scripts/deploy-local.sh              # frontend + backend
./scripts/deploy-local.sh --binary-only   # backend only
```

## Notes

- **DNS leak** is evaluated **on the router**: it compares WireGuard `dns` in UCI to effective upstream DNS (`/etc/resolv.conf` plus dnsmasq `server=` when resolv is only the local stub). It does not run in the browser.
- **WireGuard + VPN DNS:** When the tunnel is enabled and `network.wg0` has `dns`, the backend **replaces** dnsmasq `server=` with those VPN DNS IPs (even if dnsmasq previously forwarded to AdGuard `127.0.0.1#5353`). The prior dnsmasq list is saved on disk and **restored** when WireGuard is disabled so AdGuard forwarding can return.
- **Kill switch** adds a firewall **rule** `lan` → `wan` `REJECT` (see `GetKillSwitch` / `SetKillSwitch` in the backend). Verify with `nft list ruleset` on the device (look for `VPN Kill Switch`).
