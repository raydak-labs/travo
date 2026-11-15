# Testing

Unit-green does not mean it works. Several real bugs shipped green because a fixture invented a field the device never emits.

## Layers

1. **Unit** with fixtures: verbatim captured device payloads, each with provenance (device, firmware version, date) in `testdata/`. Never hand-write a payload field.
2. **Container integration**: the official `openwrt/rootfs` image (25.12; 24.10 for the opkg adapter) with real `uci`, `ubus`, `rpcd`. Catches mocks that diverge from real semantics.
3. **Hardware playbook** per release on a real device. Some behaviour is provable only there: wireless apply/confirm, lockout, DNS composition with VPN + captive + AdGuard, sysupgrade survival, AdGuard reinstall, watchdog restore.

## Rules

- Mocks reproduce real semantics: `uci set` upserts (a mock that errors on an existing section is a false failure); a mock that ignores the device name is a false pass.
- A constant asserted against itself proves nothing. Check timing budgets against measured latency.
- Test the fail-closed direction explicitly: a broken AP reads DOWN; an unknown caller is refused.
- Property tests for the DNS layer stack (any push/pop order restores base).
- Table-driven tests for caller classification using real payload shapes.
- No destructive probes (e.g. reboot POST) against an unknown build: GET-only reachability first.
- Verify the running binary is the one under test (path + version) before measuring.

## Contract tests

Scripts parse; guard paths under `/etc/travo-greenfield-guards/`; every route is in the spec; auth covers every route (walk the live route table); strict-PUT coverage; spec field names match models; no feature writes UCI outside the transaction module; generated files (OpenAPI, TS client) have no diff.

## Mock mode

`--mock` runs the whole daemon against the mock `Device`, scripted from fixtures. Frontend development needs no device.

## Running the container layer

`mise run test:integration` cross-compiles the `integration`-tagged tests and runs them against real `uci`, `apk` (`openwrt/rootfs:aarch64_generic-25.12.6`) and `opkg` (`openwrt/rootfs:x86-64-24.10.8`, amd64: no arm64 24.10 manifest exists) (needs Docker). A second step boots the image with `/sbin/init` (procd as PID 1, real ubusd/rpcd/netifd) for tests that need procd. `mise run test:package` builds the release tarball and, in a booted `aarch64_generic-25.12.6` container, runs `install.sh` (refusals, install, health on `:80`, LuCI moved, other listeners intact), restarts the container (boot link), then `--uninstall` and `--uninstall --purge`. It also runs the signature cases (ADR 0007) with throwaway minisign keys from the host (`minisign` from `mise.toml`): no `.sig` refuses (`--insecure` hint); `--insecure` installs and writes no key; a valid `.sig` installs and stores the tarball key; a tarball with another key refuses ("differs from the installed key") and installs with `--rotate-key`; a `.sig` from another key and a tampered `checksums.txt` both refuse with "signature check failed". The real release key is never used. `mise run test:port80` checks the dual-stack port swap, exact LuCI listener restoration, rollback and retry after interruption, and refusal when another process owns port 80. Both packaging suites run on arm64 and are outside `mise run check`. CI runs it on every PR that touches non-docs files: one job per arch (`integration (arm64)` on the native arm runner, `integration (amd64)`), each running `mise run test:integration <arch>` (the arm64 job then runs `mise run test:package` and `mise run test:port80`); no arg runs both locally. Container fixtures prove format and quoting only, never wireless or netifd behaviour; mark them as such in `testdata/PROVENANCE.md`.

## Hardware layer

Test router: GL-AXT1800, OpenWrt 25.12.6, `ssh root@192.168.1.1` (key auth) when the operator has it on LAN. Start from a clean OpenWrt baseline for the release playbook. Before any live test, take a `sysupgrade -b` backup to the operator's machine and keep it outside the repo. Pause competing configuration writers and restore any paused processes afterwards. Diff `/etc/config/*` and the crontab against the backup after rollback tests. Never `wifi` / `wifi up/down/reload`. GET-only first.

`hardware`-tagged tests (`internal/txn/hardware_test.go`, `internal/dnslayer/hardware_test.go`) run ON the router from Ethernet: cross-compile with `GOOS=linux GOARCH=arm64 go test -tags hardware -c`, copy with `scp -O`, run with `-test.timeout` (no `timeout` in BusyBox). They change live wireless/DNS state and rely on the watchdog/stack to restore it; take a sysupgrade backup first and pause competing configuration writers. The reboot test needs a temporary boot hook that calls `Recover`; remove it after. Results of the first run: `testdata/device/axt1800-25.12.6/SPIKES.md`.

## Hardware playbook (run before each release; console or Ethernet fallback ready)

Done on 2026-11-09 (results: `testdata/device/axt1800-25.12.6/SPIKES.md`): payload captures; watchdog restore without confirm, after daemon kill and across reboot; wrong uplink password reads `carrier:false` and rolls back; DNS layer round trip with AdGuard and captive.

Per release, each from Ethernet unless stated (issues with label `needs-hardware` add their step here when they merge):
1. On clean OpenWrt, verify LuCI login on port 80 and record the uhttpd listener values. Install the release tarball without `--port`. Set up and log in to greenfield at `http://192.168.1.1/` and verify LuCI login on port 8080. Check that `sysupgrade -l | grep travo-greenfield` lists the keep paths, including `uhttpd.orig`. Reboot and verify both UIs still answer. Run `--uninstall` and verify the original uhttpd listener values, their order and option/list form, and LuCI login on port 80.
2. Wireless change not confirmed: restores via `reload_config` alone; confirm keeps it.
3. From a phone on the AP, switch to client mode: refused with 409 `wifi_lockout_risk`.
4. Wrong uplink password: carrier stays false, rollback happens, UI says "rolled back" only after restore.
5. DNS: captive bypass over AdGuard restores base exactly; AdGuard stopped → fail-open within seconds.
6. Configuration backup (#64): sign in on the AXT1800, download the archive from System and verify it locally with `gzip -t travo-config.tar.gz` and `tar -tzf travo-config.tar.gz`. Confirm it contains router config entries. Keep the archive and its contents private.

7. Dashboard connectivity (#58): from Ethernet, verify the active interface and carrier against the kernel route and device status on a connected AXT1800. Verify the no-default-route state shows Offline and unknown uplink and carrier. Record each result without changing the router just to produce a fixture.
8. Reboot (#65): from Ethernet on the AXT1800 with a build that includes #65. GET-only first: `wget -qO- http://192.168.1.1/api/health` and record `boot_id` (B1) and `/proc/sys/kernel/random/boot_id` (K1). (a) With a wireless change applied but not confirmed, press System → Restart router → Restart: the UI shows "Confirm or roll back the pending change first." (409 `transaction_pending`), the router does not reboot, and `ls /etc/travo-greenfield-guards/` shows no `reboot-in-progress`. Roll the change back. (b) Press Restart router and confirm. The UI shows "Restarting…" within 1 s of the response, the Ethernet link drops, and within 3 minutes the UI reports "Router restarted" with a health `boot_id` different from B1. On the router: `uptime` is under 3 minutes, `/proc/sys/kernel/random/boot_id` differs from K1, `ls /etc/travo-greenfield-guards/` has no `reboot-in-progress`, `/etc/travo-greenfield/reboot.json` is absent, and `logread | grep -i 'stale guard'` shows no reboot entry. Sign in again; LuCI on :8080 and the daemon both answer. (c) Record pass or fail only, never the boot ids.

8. WireGuard tunnel status (#86): from Ethernet, configure `wg0` as in `testdata/device/axt1800-25.12.6/wireguard/PROVENANCE.md` and request `GET /api/v1/vpn/status` with a session cookie.
   - With `disabled=1`, expect `disabled`, or `configured` when a profile is selected.
   - With `disabled=0` and an unreachable endpoint, expect `no_handshake` with a null handshake age.
   - With a real endpoint after `ifup wg0`, expect `connected` with a handshake age under 180 seconds.
   Compare each result with `wg show wg0 latest-handshakes`. After disabling, run `ifup wwan` if the kernel default route is gone (device-facts §1).

**WiFi Join (#71).** Test over Ethernet with a backup and console ready. Join a known uplink with a band-compatible free radio. Record time to STA carrier and a kernel default route via the STA, then confirm and verify the route persists. Repeat without confirming and with a wrong password. Verify rollback restores the previous STA and route, and the UI reports rollback only after the transaction does. Verify the AP stays up and remains on a separate radio. Check mixed WPA2/WPA3 only with an operator-owned test network. Record the result in SPIKES.md with the SSID and BSSID redacted.

**WireGuard toggle (#87).** From Ethernet, take a fresh backup and load the operator's test profile. Enable through the VPN page and provide tunnel traffic if the peer is idle. Confirm after a new handshake. Disable and verify that wg0 is absent and that the kernel default uses the underlying uplink before confirming. Repeat enable with an unreachable controlled endpoint and verify rollback restores the original state, including the non-wg0 kernel default from a disabled baseline. Record the terminal states and elapsed times in SPIKES.md with credentials and endpoints omitted.

**WireGuard rollback (#108).** Run `scripts/test-wireguard-rollback.sh` for the real watchdog in a disposable ARM64 OpenWrt container. It uses generated keys and documentation addresses and checks the live endpoint after each restore. For release acceptance on AXT1800, take a fresh backup and have an Ethernet or console fallback ready. Start with an enabled tunnel using a controlled literal-IP endpoint. Apply a change to a second endpoint and verify it is live before triggering each restore path: explicit rollback, failed probe, confirmation timeout and startup recovery after killing the watchdog before it claims restore. Require the old endpoint in `wg show wg0 endpoints` and a final RolledBack result. Repeat with an absent or disabled baseline and require no live `wg0` link after rollback. Record outcomes and elapsed times in SPIKES.md with endpoint addresses redacted.

## Web

`pnpm -C web check`: Biome, svelte-check, Vitest (pure logic and server-rendered components), build, UI size budget. Visual and keyboard behaviour (theme, drawer, focus) is checked in headless Chrome at 390 px and 1280 px before a UI PR; attach screenshots to the PR.

`mise run screenshots` writes PNGs for UI PRs to `web/screenshots/`; `web/scripts/cdp.mjs` is the shared headless-Chrome helper for browser checks.

### Signed update hardware acceptance

On the AXT1800, use the release staging controls to verify a newer signed build, then confirm Apply update with Ethernet available. Record the configured listen port, new health version, procd PID and executable identity. Confirm the matching UI loads, the previous generation remains under `update-operation/.old`, and the update guard is gone. Exercise rollback with a controlled signed test release that cannot pass health, and confirm the prior version and UI return. Keep archives and private signing material outside the repository.
