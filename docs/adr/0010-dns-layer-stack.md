# 0010 One DNS layer stack

Status: accepted (property-tested, container-tested; device behaviour via spike S5)

Features that touch dnsmasq `server`/`noresolv`/rebind push and pop layers on one ordered record (`captive`, `vpn`, `adguard`, reserved `tailscale`). Base is captured before the first layer. Reads never mutate. Built from day one because per-feature snapshots of the same options are how the old bugs happened. See [invariants §8](../invariants.md).

Implementation (`internal/dnslayer`):
- Precedence is fixed by layer name, not push order: captive > adguard > vpn. The highest replace layer sets `server`/`noresolv`; tailscale is additive by name and appends domain-scoped servers. Any rebind-disabling layer turns rebind protection off. Effective state depends only on the active layers (property test with random payloads).
- Base keeps the `option` vs `list` shape of `server` and is restored in that shape.
- Push saves the record (intent) before staging. Last pop first marks the record `restoring`, then restores base, and deletes the record only after apply succeeded. Any failure after the first write reverts staging and keeps the guard; `Resume` finishes the interrupted operation. Read failures before any write release the guard.
- A change or `Resume` is refused while dhcp has foreign staged changes (`uci changes`): the stack never commits or reverts someone else's edits. A daemon crash mid-stage looks the same; `uci revert dhcp` or a reboot (staging lives in /tmp) clears it. Residual: a writer staging between the check and apply; the transaction module's single staging area closes that.
- `Status` is read-only and reports `pending` (guard held) and `corrupt` (unreadable record).
- Commit goes through an `Applier` (the transaction module); the stack never commits itself.
- The daemon builds one `dnslayer.Service` (record `/etc/travo-greenfield/dns-layers.json`, applier `txn.Manager.Apply`, `AfterApply` = the shared connectivity probe's `Invalidate`); every layer user receives that instance.

## Captive layer (#130)

The `captive` layer carries the resolvers netifd learned on the interface whose `l3_device` carries the kernel default route, with `DisableRebind` (hotel portals answer with RFC1918, device-facts §7). No resolvers or no default route refuses before any write. It replaces whatever is below it (AdGuard, VPN) by precedence, so the old per-feature backups of WAN DNS fields and the AdGuard upstream are unnecessary: nothing outside dnsmasq is touched. The 5-minute auto-restore deadline is memory-only on the monotonic clock: no flash write per bypass, and wall-clock jumps (time sync) cannot shorten it. A daemon restart with the layer still on the stack arms a fresh 5 minutes on the first worker tick (15 s), so a bypass can outlive its deadline by at most one restart window, never indefinitely. A failed restore keeps the dnslayer guard; the dashboard shows it as pending and offers only "Retry DNS restore", which runs `Resume` and then pops the layer (an explicit POST; the worker never resumes).

## VPN layer (#88)

The `vpn` layer is owned by a reconciler (`internal/vpndns`, worker every 15 s) on the same shared `dnslayer.Service` instance, never by the WireGuard transaction. It pushes the selected profile's `DNS =` resolvers with `noresolv=1` once the tunnel is `connected` and no transaction awaits confirmation, and pops on `disabled`/`configured` (also during a pending transaction: popping returns to uplink DNS). Transient tunnel states and unreadable inputs change nothing. While the stack guard is held after a failed change the reconciler does nothing; the explicit retry is the same `Resume` path as the captive restore. Rules and verify order: invariants §8.
