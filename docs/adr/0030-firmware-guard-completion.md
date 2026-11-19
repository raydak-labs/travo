# 0030 A finished firmware upgrade releases its guard

Status: accepted

Amends "firmware-upgrade guards are never removed" in [invariants](../invariants.md) §1 and
the firmware-upgrade line of §5. Factory reset is unchanged.

## Context

The firmware upgrade (#198) writes `firmware-upgrade-in-progress` before it schedules
`sysupgrade` and never removes it, so an interrupted flash stays visible. With "keep
settings" the keep-list carries `/etc/travo-greenfield-guards/` across the flash, so after a
flash that worked the guard is still there. Every feature that refuses during a terminal
operation (uplink reconnect, WiFi schedule, slide switch, interface restart, reboot from the
UI, factory reset, the next upgrade) then stayed off until the operator ran `install.sh`.
A successful upgrade looked like a stuck one.

## Decision

- When Apply writes the guard it records, in the guard file's `detail`, the current boot id
  (`/proc/sys/kernel/random/boot_id`), the running firmware revision (`ubus call system
  board` → `release.revision`) and the image's SHA-256. A value that cannot be read is left
  empty; the upgrade is not refused for it.
- Once at daemon start, before any guarded feature runs, the firmware service reads the
  guard. It releases it only when the recorded boot id and revision are both known, the boot
  id differs from the running one AND the running revision differs from the recorded
  pre-flash one: the router rebooted into other firmware, so the flash finished.
- Released, the daemon raises the alert `firmware_upgrade_completed` (severity `info`, shown
  in the banner) for the rest of that daemon run. It lives in memory, so it shows once.
- Everything else keeps the guard as before: same boot (the daemon restarted, the flash may
  still be pending), same revision (the flash was interrupted or failed and the old image
  booted), or anything unreadable or missing (fail closed, also a guard written before this
  record). `install.sh` stays the explicit retry path.

## Consequences

- Reflashing the image that already runs (same revision) never releases the guard; the
  operator runs `install.sh`, as before.
- A flash without "keep settings" wipes the guard with the overlay; nothing changes there.
- The guard file gains an optional `detail` object; older daemons ignore it.
