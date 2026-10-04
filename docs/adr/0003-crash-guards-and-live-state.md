---
title: "ADR 0003: Crash guards and automated live-state changes"
status: Accepted
date: 2026-05-14
updated: 2026-10-04
tags: [adr, safety, travo, guards, operations]
---

# ADR 0003: Crash guards and automated live-state changes

## Status

Accepted.

## Context

The router runs Travo as a **long-lived process**. Automated jobs (failover apply, background WiFi recovery, band switching, firmware flash, …) can **commit UCI**, **restart services**, or **change routes** while the process could crash mid-flight. Without a durable marker, the next boot might repeat a half-finished operation or leave the system in an ambiguous state.

This ADR is distinct from **transaction snapshots** (e.g. VPN DNS JSON, captive DNS JSON) which store *configuration deltas* for restore. Crash guards answer: **“Should this feature refuse to start because a previous run died?”**

## Decision

### 1. Guard file contract

Any automated action that **materially changes live system state** and is **unsafe to blindly retry** after a crash must:

1. **Write** a guard file under **`/etc/trafo/<feature>-in-progress`** (or a clearly named variant) **before** the dangerous segment.
2. On **startup** (or on the next run of the guarded operation), if the guard exists, **skip** repeating the operation until an operator clears it.
3. **Remove** the guard file **only after** the operation completes successfully end-to-end.
4. Treat **`deploy-local.sh`** / manual redeploy as the **explicit recovery** path that may clear stuck guards — see the authoritative list in §2.

Notes on step 2, as implemented:

- Failover and band switching both log the skip once at startup
  (`failover: crash guard found at … — automatic switching is disabled`,
  `band-switching: crash guard found at … — skipping auto switch`). `SystemService.LogStaleCrashGuards`
  warns for the four system guards, and the generated wireless toggle helper logs through
  `logger -t travo-wifi-toggle`. Failover used to skip silently, which made "failover never fires"
  undiagnosable; it now logs. **Do not assume a visible log line exists** — a guard written after
  startup is still silent — so check the filesystem.
- Where the guard directory is not writable (dev host, unit tests, broken install),
  the guard is **not skipped**: it is written to a temp fallback directory and an
  `ERROR:` line is logged saying that a device-side power loss is *not* protected
  (`WifiService.resolveGuardDir`, `SystemService.guardDirOrDefault`).

### 2. Authoritative guard list

This table is the source of truth for which files exist on the device and what each one protects.
`scripts/deploy-local.sh` clears the guards listed here, so adding a row here means adding it to that
script; leaving a guard out of the script means the documented recovery path does not exist for that
feature (a stuck guard permanently disables it).

| Guard path | Owning file | What it protects |
| ---------- | ----------- | ---------------- |
| `/etc/trafo/failover-in-progress` | `services/failover_service.go` (`:23`) | mwan3 policy generation + apply, the staged `StartApply` → verify → `Confirm` window, and the whole `SetConfig` sequence |
| `/etc/trafo/band-switch-in-progress` | `services/band_switching_service.go` (`:21`) | automated dual-band down/up switching of the client between radios |
| `/etc/trafo/captive-dns-in-progress` | `services/captive_service.go` (`:25`) | the dnsmasq/AdGuard DNS bypass; doubles as the serialized pre-bypass backup **and** the “bypass active” marker (see ADR 0001) |
| `/etc/trafo/captive-wwan-bounce-in-progress` | `services/captive_autoaccept.go` (`:31`) | the `ifdown wwan` / `ifup wwan` DHCP-lease bounce; removed only after a fresh lease came back |
| `/etc/trafo/vpn-in-progress` | `services/vpn_service.go` (`:618`) | WireGuard enable: tunnel commit, verify, `wg0` firewall zone + `lan→wg0` forwarding, dnsmasq VPN-resolver forwarding, and their rollback |
| `/etc/trafo/usbtether-in-progress` | `services/usb_tethering_service.go` (`:25`) | the `network.usbtether` interface, its addition to the **`wan`** firewall zone, and the `ifup`/`ifdown` |
| `/etc/trafo/wifi-toggle-in-progress` | the generated helper `services/wifi_toggle_script.go` (`:34`, written to `/usr/libexec/travo-wireless-toggle.sh`) | flipping `wireless.*.disabled` + rpcd `apply`/`confirm` for the WiFi schedule and the button toggle |
| `/etc/trafo/mac-in-progress` | `services/wifi_service.go` (`defaultGuardDir` `:103`, `guardPath` `:602`, feature `mac`) | the custom STA MAC sequence: UCI `macaddr` write, staged apply, live `ip link` down/address/up |
| `/etc/trafo/restore-in-progress` | `services/system_service.go` (`:94`) | restoring a backup tarball over `/` |
| `/etc/trafo/firmware-upgrade-in-progress` | `services/system_service.go` (`:95`) | `sysupgrade` — **deliberately not removed**; the marker must survive the reboot so an interrupted flash is discoverable |
| `/etc/trafo/factory-reset-in-progress` | `services/system_service.go` (`:96`) | `firstboot -y && reboot`; also not removed, because the device reboots before anyone could clear it |
| `/etc/trafo/system-config-in-progress` | `services/system_service.go` (`:97`) | committing `/etc/config/system` (timezone, hostname, NTP servers); protects the **commit**, not an apply — these settings deliberately need a reboot to take effect |
| `/etc/trafo/pkg-install-in-progress` | `services/service_manager.go` (`:264`) | package install/remove, which rewrites init scripts, `/etc/config` and kernel modules |
| `/etc/trafo/autoreconnect-crash-guard` | generated auto-reconnect script, `services/wifi_reconnect.go` (`:64`) | the `wifi up` attempt in the bounded auto-reconnect path; survives a driver crash, so every later cron tick is a no-op |
| `/etc/trafo/autoreconnect-failcount` | same script (`wifi_reconnect.go` `:65`) | **not a guard**: the `MAX_FAIL=5` retry counter for a broken saved network. A successful reconnect clears it. Deliberately **not** cleared by a redeploy. |

**One directory (resolved 2026-09-28).** Guards used to be split across `/etc/trafo/` and
`/etc/travo/` — and even a single feature straddled both (`captive-dns-in-progress` in one,
`captive-wwan-bounce-in-progress` in the other). A guard written to one directory was invisible
to a check that only looked at the other, and `deploy-local.sh` could not clear all of them, so
a stale guard permanently disabled a feature with no log line. **Every crash guard is now
written to `/etc/trafo/`**; `internal/services/guards_test.go` scans the service
sources and fails if a `-in-progress` / `-crash-guard` / `-failcount` path reappears under
`/etc/travo/`. Non-guard state (aliases, WiFi priorities, repeater options, the bbolt store)
stays in `/etc/travo/` — moving it would lose user data on upgrade. `deploy-local.sh` clears
both directories so a device upgraded from an older build still recovers.

Other `/etc/trafo/*.json` files store **config** or **snapshots** (VPN DNS, failover backup, port forwards,
MAC policies, split tunnel, …) without necessarily being crash guards — distinguish by whether startup
logic consults them as **“abort if present”** signals.

### 3. Related patterns

- **Wireless** safety is primarily **LuCI-style rollback** (ADR 0002), not the same as a crash guard, but overlaps philosophically.
- **Autoreconnect** shell logic uses a crash guard **and** a fail-count file (§2, last two rows) — bounded recovery, documented in code comments.
- **Startup AP repair** (`main.go` → `WifiService.EnsureAPRunning`) commits `wireless` fixes with **no guard**, deliberately: there is no browser in the loop to confirm an rpcd rollback, so the repair is committed and the operator applies via LuCI “Save & Apply” or a reboot. The reasoning lives in the code comment at the call site, not here.
- **Direct `uci commit` + reload** paths that bypass rpcd `apply`/`confirm` still need a guard: the WireGuard `wg0` zone and the USB-tether `wan` zone membership are committed and reloaded directly (see ADR 0004 §5).

### 3.1 Bounded recovery: `network reload` after a refused apply

The rule everywhere else is that **applying a user's wireless changes must not run `wifi`,
`wifi up` or `wifi reload`** — they are the classic ath11k/IPQ6018 driver-crash trigger and
an automated one has no rollback. **Recovery is the single, explicitly bounded exception, and
it is stated in one place so it cannot be generalised:**

- **What it is.** When `WifiService.ConfirmApply`'s probe refuses the apply (ADR 0002 §5),
  `UCIApplyConfirm.Rollback` restores the file-level snapshot taken before the mutation
  committed, cancels rpcd's rollback window, and then issues **`ubus call uci reload_config`**
  (rpcd re-reads `/etc/config`) followed by **`ubus call network reload`** (netifd re-reads
  `/etc/config/network` **and** `/etc/config/wireless`).
- **What it is not.** `network reload` is a netifd configuration reload, **not** a driver
  reload: it never calls `wifi`, `wifi up` or `wifi reload`. Those three remain forbidden on
  this path, and on every other one.
- **Why here and only here.** There is no browser left to confirm and no operator watching:
  the config on disk is one that Travo itself has just proved cannot work. Leaving the device
  running it would strand the operator off their router, which is the exact outcome a crash
  guard exists to prevent — so this is recovery, and recovery is guarded by making the
  previous config real again rather than by doing nothing.
- Both ubus objects and their zero-argument signatures were verified on the device with
  `ubus -v list uci` (`reload_config`) and `ubus -v list network` (`reload`).

## Consequences

- New **goroutines**, **cron hooks**, or **init**-triggered Travo paths that mutate connectivity must either use this guard pattern or be proven safe to retry idempotently without user intervention.
- Documentation and on-call playbooks should list **known guard paths** (§2) when debugging “feature won’t run” reports. “Feature won’t run, no log line” should make `ls /etc/trafo` the first check (plus `/etc/travo` on a device upgraded from a build that predates the unification).

## References

- `docs/architecture.md` §4, §6.2
- `backend/internal/services/system_service.go` — the four system guards, `LogStaleCrashGuards`
- `backend/internal/services/wifi_toggle_script.go` — the generated toggle helper and its guard
- `backend/internal/services/wifi_reconnect.go` — auto-reconnect guard + fail count
- `backend/internal/services/wifi_service.go`, `service_manager.go` — guard paths (now `/etc/trafo/`)
- `backend/internal/services/uci_apply.go` — `Snapshot`/`Rollback`, the bounded-recovery `network reload` (§3.1)
- [ADR 0002](./0002-wireless-model-and-luci-apply.md) — apply/confirm and the file-level rollback (§5, §5.0)
- `backend/internal/services/failover_service.go`, `band_switching_service.go`,
  `captive_service.go`, `captive_autoaccept.go`, `vpn_service.go`,
  `usb_tethering_service.go` — guard paths
- `scripts/deploy-local.sh` — the redeploy recovery path
