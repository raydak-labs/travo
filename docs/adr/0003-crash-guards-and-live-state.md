---
title: "ADR 0003: Crash guards and automated live-state changes"
description: The crash-guard contract for automated live-state changes: what is guarded in /etc/trafo, where ordinary state lives, and how a guard is cleared.
status: Accepted
date: 2026-05-14
updated: 2026-10-25
tags: [adr, safety, travo, guards, operations]
---

# ADR 0003: Crash guards and automated live-state changes

## Status

Accepted.

## Context

The router runs Travo as a **long-lived process**. Automated jobs (failover apply, background
WiFi recovery, band switching, firmware flash, …) commit UCI, restart services or change routes,
and the process can die mid-flight. Without a durable marker the next run may repeat a
half-finished operation or leave the device in an ambiguous state.

Crash guards are distinct from **snapshots** (the dnsmasq layer record, the failover mwan3
backup), which store configuration to restore. A guard answers one question: *should this
feature refuse to run because a previous run died?*

## Decision

### 1. Guard contract

Any automated action that **materially changes live system state** and is **unsafe to blindly
retry** after a crash:

1. **Writes** `/etc/trafo/<feature>-in-progress` **before** the dangerous segment.
2. On startup, or on the next run of the guarded operation, **skips** the operation while the
   guard exists. Package installs refuse outright (`guard.Guard.Create`).
3. **Removes** the guard **only after** the operation completed end-to-end.
4. Treats `scripts/deploy-local.sh` and `scripts/install.sh` as the **explicit recovery** path:
   both clear the guards listed in §2.

All Go code goes through `backend/internal/guard`. `guard.In(dir, feature)` names a guard,
`Write`/`Mark` create it atomically (temp file + rename, `0600`), `Present`/`Read` inspect it and
`Clear` removes it. The directory is `guard.DefaultDir` (`/etc/trafo`); mock mode and tests
redirect it with `guard.SetDir`, so no test can touch the real directory. There is no silent
fallback: a guard that cannot be written fails the operation it was meant to protect.

Failover and band switching log a stuck guard once at startup, and
`SystemService.LogStaleCrashGuards` warns for the system guards. A guard written after startup is
still silent, so check the filesystem first when a feature "never runs".

### 2. Authoritative guard list

This table is the source of truth for the guard files on the device. Both recovery scripts clear
the guards listed here; adding a row means adding it to them, or the documented recovery path
does not exist for that feature.

| Guard path | Owner | What it protects |
| ---------- | ----- | ---------------- |
| `/etc/trafo/failover-in-progress` | `FailoverService.SetConfig` | mwan3 generation + apply, the staged apply → verify → confirm window |
| `/etc/trafo/band-switch-in-progress` | `BandSwitchingService.doSwitch` | automated move of the client between radios |
| `/etc/trafo/captive-dns-in-progress` | `CaptiveService.BypassDNS` | the DNS bypass; doubles as the pre-bypass backup and the "bypass active" marker (ADR 0001) |
| `/etc/trafo/captive-wwan-bounce-in-progress` | `CaptiveService.bounceActiveWwanDHCP` | the `ifdown`/`ifup wwan` lease bounce |
| `/etc/trafo/vpn-in-progress` | `VpnService.ToggleWireguard` | WireGuard enable/disable: tunnel, `wg0` zone, DNS layer, rollback |
| `/etc/trafo/usbtether-in-progress` | `USBTetheringService` | the `usbtether` interface, its `wan` zone membership, `ifup`/`ifdown` |
| `/etc/trafo/wifi-toggle-in-progress` | generated helper `wirelessToggleScript` | flipping `wireless.*.disabled` + rpcd apply/confirm for schedule and button |
| `/etc/trafo/mac-in-progress` | `WifiService.SetMACAddress` | UCI `macaddr`, staged apply, live `ip link` down/address/up |
| `/etc/trafo/restore-in-progress` | `SystemService.RestoreBackup` | restoring a backup tarball over `/` |
| `/etc/trafo/firmware-upgrade-in-progress` | `SystemService` firmware upgrade | `sysupgrade`; **never removed**, so an interrupted flash stays discoverable |
| `/etc/trafo/factory-reset-in-progress` | `SystemService.FactoryReset` | `firstboot -y && reboot`; never removed, the device reboots first |
| `/etc/trafo/system-config-in-progress` | `SystemService` system config commit | committing `/etc/config/system`; the commit, not an apply |
| `/etc/trafo/pkg-install-in-progress` | `ServiceManager` install/remove | package operations that rewrite init scripts, `/etc/config` and modules |
| `/etc/trafo/autoreconnect-crash-guard` | generated script `reconnectScriptContent` | the bounded `wifi up` in auto-reconnect |
| `/etc/trafo/autoreconnect-failcount` | same script | **not a guard**: the `MAX_FAIL=5` retry counter; never cleared by a redeploy |

Non-guard state (aliases, WiFi priorities, repeater options, the bbolt store) lives in
`/etc/travo/`. `internal/docsgate/guards_test.go` fails if a guard-shaped path is declared there.

### 3. Related patterns

- **Wireless** safety is primarily LuCI-style rollback (ADR 0002), not a crash guard.
- **Startup AP repair** (`WifiService.EnsureAPRunning`) commits with **no guard**, deliberately:
  no browser can confirm an rpcd rollback at boot, so the repair is committed and applied by the
  operator via LuCI or a reboot.
- **Direct `uci commit` + reload** paths that bypass rpcd still need a guard: the `wg0` zone and
  the USB-tether `wan` membership (ADR 0004 §5).

### 3.1 Bounded recovery: `network reload` after a refused apply

Applying a user's wireless change must not run `wifi`, `wifi up` or `wifi reload`; they are the
ath11k/IPQ6018 driver-crash trigger. Recovery is the single bounded exception: when
`WifiService.ConfirmApply`'s probe refuses an apply (ADR 0002 §5), `UCIApplyConfirm.Rollback`
restores the pre-mutation file snapshot, cancels rpcd's rollback window, and issues
`ubus call uci reload_config` and `ubus call network reload`. That is a netifd configuration
reload, not a driver reload. Both signatures were verified on the device with
`ubus -v list uci` and `ubus -v list network`.

## Why

- A marker on disk survives the crash it describes; in-memory state does not.
- One package means one directory, one write path and one place to redirect in tests. Guards
  were once split across `/etc/trafo` and `/etc/travo`, and a guard in one was invisible to a
  check in the other, so a stale guard disabled a feature with no log line.
- Refusing rather than retrying keeps a human in the loop for exactly the cases automation
  already got wrong once.

## Consequences and invariants

- New goroutines, cron hooks or init paths that mutate connectivity use a guard, or are proven
  safe to retry idempotently.
- "Feature won't run, no log line" makes `ls /etc/trafo` the first check.
- A guard that cannot be written aborts the change; it never degrades to "unguarded".

## Deliberate ceilings

- Guards are per feature, not per operation instance: two different operations of one feature
  share a marker.
- Nothing clears a guard automatically; recovery is a redeploy, a reinstall or `rm`.
- Shell helpers spell the directory out (they cannot import Go); `TestShellHelpersUseTheGuardDir`
  pins it.
