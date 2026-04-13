---
title: On-device validation
description: Integration shell scripts plus full manual playbook for router API, WiFi, VPN, evidence capture.
updated: 2026-04-13
tags: [docs, testing, integration, openwrt]
---

# On-device validation

Router default `192.168.1.1` unless noted. Evidence often goes under `./tmp/`. Replace example SSIDs, passwords, and paths in the playbook with your lab values.

## Quick script checks

These scripts exercise the **live router** (default `192.168.1.1`). They use the HTTP API and optional SSH. Use the same password as the travel-gui login (often `admin` on lab images).

### Scripts (from repo root)

| Script | Purpose |
|--------|---------|
| `test/integration/integration-vpn-wireguard-toggle.sh` | WireGuard enable → disable; router must keep internet (`wget` connectivity check). |
| `test/integration/integration-vpn-dns-killswitch.sh` | DNS leak endpoint + kill switch GET; optional `--ssh-verify`, `--enable-killswitch`. |
| `test/integration/integration-device.sh` | Broader smoke test (health, WiFi connect/disconnect); needs `test/integration/.wifi_pass`. |

#### DNS leak + kill switch (quick)

```sh
./test/integration/integration-vpn-dns-killswitch.sh
```

With UCI snapshot over SSH:

```sh
./test/integration/integration-vpn-dns-killswitch.sh --ssh-verify
```

Briefly enable kill switch, confirm API and UCI, then disable:

```sh
./test/integration/integration-vpn-dns-killswitch.sh --enable-killswitch --ssh-verify
```

#### WireGuard toggle (regression)

```sh
./test/integration/integration-vpn-wireguard-toggle.sh --login-password 'your-password'
```

### Deploying a new backend before testing

```sh
./scripts/deploy-local.sh              # frontend + backend
./scripts/deploy-local.sh --binary-only   # backend only
```

### Notes

- **DNS leak** is evaluated **on the router**: it compares WireGuard `dns` in UCI to effective upstream DNS (`/etc/resolv.conf` plus dnsmasq `server=` when resolv is only the local stub). It does not run in the browser.
- **WireGuard + VPN DNS:** When the tunnel is enabled and `network.wg0` has `dns`, the backend **replaces** dnsmasq `server=` with those VPN DNS IPs (even if dnsmasq previously forwarded to AdGuard `127.0.0.1#5353`). The prior dnsmasq list is saved on disk and **restored** when WireGuard is disabled so AdGuard forwarding can return.
- **Kill switch** adds a firewall **rule** `lan` → `wan` `REJECT` (see `GetKillSwitch` / `SetKillSwitch` in the backend). Verify with `nft list ruleset` on the device (look for `VPN Kill Switch`).


## Full real-device playbook

This document describes how to validate real functionality on an OpenWrt device
(`192.168.1.1`) and store evidence under local `./tmp/` for comparison.

### Scope

- Device-level integration checks (not unit tests)
- Network state transitions (offline -> online)
- Backend API + runtime service behavior
- UCI/runtime persistence across restart/reboot

### Assumptions

- Test machine is connected to router via LAN (stable management path)
- SSH access is available as `root@192.168.1.1`
- Project root is current working directory
- SSID under test: `Cappuxinno`
- WiFi password is stored in local file `test/integration/.wifi_pass`

### Test Artifacts

Use one timestamped folder per run:

```sh
TS="$(date +%Y%m%d-%H%M%S)"
RUN_DIR="tmp/integration-$TS"
mkdir -p "$RUN_DIR"/{before,after,logs}
echo "Run dir: $RUN_DIR"
```

Store all command output in this folder so runs can be diffed later.

### Pre-Test Baseline Capture

#### 1) Device/system snapshot

```sh
ssh root@192.168.1.1 '
  echo "=== date ==="; date
  echo "=== release ==="; (cat /etc/openwrt_release 2>/dev/null || cat /etc/os-release 2>/dev/null)
  echo "=== uname ==="; uname -a
  echo "=== df -h ==="; df -h
  echo "=== services ==="; /etc/init.d/travo status 2>/dev/null || true
  echo "=== uhttpd ports ==="; uci get uhttpd.main.listen_http; uci get uhttpd.main.listen_https
' > "$RUN_DIR/before/system.txt"
```

#### 2) UCI snapshot for comparison

```sh
ssh root@192.168.1.1 '
  uci show network
  uci show wireless
  uci show firewall
  uci show dhcp
  uci show travo 2>/dev/null || true
' > "$RUN_DIR/before/uci-show.txt"
```

Optional full config archive:

```sh
ssh root@192.168.1.1 'tar -czf - -C /etc/config network wireless firewall dhcp system uhttpd travo 2>/dev/null || true' \
  > "$RUN_DIR/before/etc-configs.tar.gz"
```

#### 3) Baseline API/service reachability

```sh
curl -sS -m 8 -D "$RUN_DIR/before/health.headers" -o "$RUN_DIR/before/health.body" \
  "http://192.168.1.1/api/health" || true
curl -sS -m 8 -o /dev/null -w "%{http_code}\n" "http://192.168.1.1/" \
  > "$RUN_DIR/before/ui.http_code" || true
curl -sS -m 8 -o /dev/null -w "%{http_code}\n" "http://192.168.1.1:8080/" \
  > "$RUN_DIR/before/luci.http_code" || true
```

### Core Integration Scenario: Offline -> Connect STA -> Online

Goal:
- Verify internet is unavailable when no STA is connected
- Connect STA to `Cappuxinno`
- Verify internet + WAN source status become healthy

#### 4) Ensure no STA connected

Use backend API when available, otherwise UCI check:

```sh
ssh root@192.168.1.1 '
  echo "=== wireless sta sections ==="
  uci show wireless | grep -E "mode=.sta" || true
  echo "=== wwan status ==="
  ubus call network.interface.wwan status 2>/dev/null || true
' > "$RUN_DIR/logs/sta-before.txt"
```

If a STA section is actively connected, disconnect via UI/API first, then re-check.

#### 5) Verify internet does not work (negative test)

Run from router:

```sh
ssh root@192.168.1.1 '
  wget -qO- --timeout=8 http://connectivitycheck.gstatic.com/generate_204 >/dev/null
  echo "exit_code=$?"
' > "$RUN_DIR/logs/offline-check.txt" || true
```

Expected:
- Non-zero exit code or failed fetch when no upstream is connected

#### 6) Connect wireless to SSID `Cappuxinno`

Read password from local `test/integration/.wifi_pass`:

```sh
WIFI_PASS="$(tr -d "\n" < test/integration/.wifi_pass)"
```

Preferred path: connect through app/API (so real backend flow is tested).
Important: wireless mutating calls return an `apply.token` that must be confirmed
via `/api/v1/wifi/apply/confirm` before continuing with the next mutating step.

```sh
LOGIN_JSON="$(curl -sS -m 8 -X POST "http://192.168.1.1/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"password":"admin"}')"
echo "$LOGIN_JSON" > "$RUN_DIR/logs/login.json"
TOKEN="$(printf '%s' "$LOGIN_JSON" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')"

# Optional but recommended: clean disconnect before connect
DISC="$(curl -sS -m 20 -X POST "http://192.168.1.1/api/v1/wifi/disconnect" \
  -H "Authorization: Bearer $TOKEN")"
echo "$DISC" > "$RUN_DIR/logs/wifi-disconnect.json"
DISC_APPLY_TOKEN="$(printf '%s' "$DISC" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')"
if [ -n "$DISC_APPLY_TOKEN" ]; then
  curl -sS -m 20 -X POST "http://192.168.1.1/api/v1/wifi/apply/confirm" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d "{\"token\":\"$DISC_APPLY_TOKEN\"}" \
    > "$RUN_DIR/logs/wifi-disconnect-confirm.json"
fi

CONNECT_JSON="$(curl -sS -m 30 -X POST "http://192.168.1.1/api/v1/wifi/connect" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"ssid\":\"Cappuxinno\",\"password\":\"${WIFI_PASS}\",\"encryption\":\"psk2\"}")"
echo "$CONNECT_JSON" > "$RUN_DIR/logs/wifi-connect-response.json"
CONN_APPLY_TOKEN="$(printf '%s' "$CONNECT_JSON" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')"
if [ -n "$CONN_APPLY_TOKEN" ]; then
  curl -sS -m 20 -X POST "http://192.168.1.1/api/v1/wifi/apply/confirm" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d "{\"token\":\"$CONN_APPLY_TOKEN\"}" \
    > "$RUN_DIR/logs/wifi-connect-confirm.json"
fi
```

#### 7) Verify connection now works

Router-level internet check:

```sh
ssh root@192.168.1.1 '
  wget -qO- --timeout=8 http://connectivitycheck.gstatic.com/generate_204 >/dev/null
  echo "exit_code=$?"
' > "$RUN_DIR/logs/online-check.txt"
```

WWAN state and default route checks:

```sh
ssh root@192.168.1.1 '
  echo "=== wwan ubus ==="
  ubus call network.interface.wwan status 2>/dev/null || true
  echo "=== routes ==="
  ip route
' > "$RUN_DIR/logs/wwan-status-after.txt"
```

Backend state checks:

```sh
curl -sS -m 8 -H "Authorization: Bearer <TOKEN>" \
  "http://192.168.1.1/api/v1/network/status" \
  > "$RUN_DIR/logs/network-status-after.json"
curl -sS -m 8 -H "Authorization: Bearer <TOKEN>" \
  "http://192.168.1.1/api/v1/wifi/connection" \
  > "$RUN_DIR/logs/wifi-connection-after.json"
```

Expected:
- Router can reach external endpoint
- `wwan` has IPv4 address
- Backend shows WAN source connected/healthy

### Additional Real-Functionality Suites

### A) WireGuard basic integration

1. Configure/import valid WireGuard profile via UI/API.
2. Enable tunnel.
3. Validate:
   - `wg show` has latest handshake and transfer counters
   - route policy is as expected
   - internet still works

Evidence:

```sh
ssh root@192.168.1.1 'wg show; ip route; ip rule' > "$RUN_DIR/logs/wireguard.txt"
```

### B) AdGuard DNS integration

1. Ensure AdGuard service is running.
2. Validate DNS resolution path from router and LAN client.
3. Confirm blocked domain behavior (if blocklist configured).

Evidence:

```sh
ssh root@192.168.1.1 '
  /etc/init.d/adguardhome status 2>/dev/null || true
  nslookup openwrt.org 127.0.0.1 || true
  nslookup openwrt.org 192.168.1.1 || true
' > "$RUN_DIR/logs/adguard.txt"
```

### C) Guest network isolation

1. Enable guest WiFi.
2. Connect a test client.
3. Validate:
   - guest client receives IP in guest subnet
   - guest client can access internet
   - guest client cannot access LAN/private IP targets

Evidence:
- client-side ping/curl results
- router firewall/lease snapshot:

```sh
ssh root@192.168.1.1 '
  uci show firewall | grep guest || true
  cat /tmp/dhcp.leases
' > "$RUN_DIR/logs/guest.txt"
```

### D) Recovery and persistence after restart/reboot

1. Trigger network disable/enable flow under test.
2. Reboot device:

```sh
ssh root@192.168.1.1 'reboot'
```

3. Wait for SSH return; then verify:
   - app reachable on `:80`
   - LuCI reachable on `:8080`
   - expected interfaces and firewall state restored
   - no crash-guard files stuck unexpectedly

```sh
ssh root@192.168.1.1 '
  ls -la /etc/travo 2>/dev/null || true
  uci show network
  uci show wireless
  uci show firewall
' > "$RUN_DIR/logs/post-reboot.txt"
```

### Post-Test Snapshot + Comparison

Capture the same artifacts again:

```sh
ssh root@192.168.1.1 '
  uci show network
  uci show wireless
  uci show firewall
  uci show dhcp
  uci show travo 2>/dev/null || true
' > "$RUN_DIR/after/uci-show.txt"
```

Diff before/after:

```sh
diff -u "$RUN_DIR/before/uci-show.txt" "$RUN_DIR/after/uci-show.txt" > "$RUN_DIR/uci.diff" || true
```

Also save backend logs:

```sh
ssh root@192.168.1.1 'logread -e travo -l 500' > "$RUN_DIR/after/logread-travo.txt"
```

### Troubleshooting Notes (from real run)

- If `wifi/connect` fails with `uci apply ... (Permission denied)`, check whether a
  previous mutating call returned `apply.pending=true` and was not confirmed yet.
  Confirm that token first, then retry.
- If you see `session login` with empty `password`, the backend lost the sealed rpcd
  credential (e.g. first boot, `/etc/travo/` wiped, or `auth.json` JWT secret replaced
  without a matching `rpcd-login.sealed`). Log in once via `/api/v1/auth/login` to recreate
  the seal file; a valid JWT alone does not carry the root password.
- If offline/online checks use `wget`, OpenWrt commonly reports failures as
  `exit_code=4` (`Operation not permitted` / transport-level failure).
- During connect, `wwan` may show `"pending": true` briefly before DHCP completes.
  Wait a few seconds and re-check `ubus call network.interface.wwan status`.

- WireGuard profile activation can currently report `"status":"ok"` while no
  `network.wg0` section exists. In this state:
  - `/api/v1/vpn/wireguard/status` returns an empty interface payload
  - `wg show` is empty
  - `ifstatus wg0` reports interface not found
  This indicates profile persistence works, but runtime interface creation/apply
  is incomplete and needs implementation follow-up.

- AdGuard service can be running while backend reports `"installed": false`.
  Also, enabling "AdGuard DNS" may set `dnsmasq` forwarding to `127.0.0.1#5353`
  before AdGuard DNS upstream is properly configured, causing `nslookup` timeouts.
  Validate both process state and real DNS resolution, not only API toggle success.

### Manual Run Record (2026-03-23)

Run artifacts:

- `tmp/integration-manual-20260323-163009/`

Executed:

- Installed WireGuard service via API (`/api/v1/services/wireguard/install`)
- Installed LuCI plugin `luci-proto-wireguard` via `apk`
- Added and activated profile from
  `test/integration/wireguard-profiles/privado.ams-033.conf`
- Toggled WireGuard enable via API
- Installed/started AdGuard via API and tested DNS toggle

Observed outcomes:

- **WireGuard packages/plugin install:** success (`wireguard-tools`,
  `kmod-wireguard`, `luci-proto-wireguard` present)
- **WireGuard profile add/activate API:** returns success
- **WireGuard runtime:** failed (no `wg0` interface created, empty `wg show`)
- **AdGuard process/UI:** running (`/usr/bin/AdGuardHome` on `:3000`)
- **AdGuard DNS integration:** failed as a functional DNS path test
  (`nslookup` timeout after enabling DNS toggle)

Tracked follow-up implementation plan:

- [WireGuard + AdGuard Out-Of-Box Fix Plan](./plans/wireguard-adguard-oob-fix-plan.md)

### Input Needed For Full Validation

No additional product requirement input is needed for baseline behavior:
both WireGuard and AdGuard are expected to work out-of-the-box.
Remaining work is tracked as backend implementation gaps in:

- [WireGuard + AdGuard Out-Of-Box Fix Plan](./plans/wireguard-adguard-oob-fix-plan.md)

### Pass/Fail Criteria

A run is considered PASS when all are true:

- App/API reachable (`/api/health` returns `{"status":"ok"}`)
- LuCI reachable on `:8080`
- Offline negative test fails as expected when STA absent
- Online test succeeds after connecting to `Cappuxinno`
- Backend/interface state reflects connected upstream
- No unexpected service crashes or reboot loops
- UCI/runtime deltas are explainable and intentional

### Suggested Future Automation

- Use `test/integration/integration-device.sh` to execute this core flow end-to-end and
  generate `tmp/integration-<timestamp>/result.json`.
- Parse outputs and emit machine-readable summary (`PASS/FAIL`) to `tmp/.../result.json`.
- Integrate in CI as optional/manual job for hardware-in-the-loop validation.
