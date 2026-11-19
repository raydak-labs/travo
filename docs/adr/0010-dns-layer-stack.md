# 0010 One DNS layer stack

Status: accepted (property-tested, container-tested; device behaviour via spike S5)

Features that touch dnsmasq `server`/`noresolv`/rebind push and pop layers on one ordered record (`captive`, `vpn`, `adguard`, `tailscale`). Base is captured before the first layer. Reads never mutate. Built from day one because per-feature snapshots of the same options are how the old bugs happened. See [invariants §8](../invariants.md).

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

## Tailscale layer (#179)

The additive `tailscale` layer carries `/<MagicDNSSuffix>/100.100.100.100`, owned by a reconciler in `internal/tailscale` on the same shared instance; see [ADR 0017](0017-tailscale.md).

## VPN layer (#88)

The `vpn` layer is owned by a reconciler (`internal/vpndns`, worker every 15 s) on the same shared `dnslayer.Service` instance, never by the WireGuard transaction. It pushes the selected profile's `DNS =` resolvers with `noresolv=1` once the tunnel is `connected` and no transaction awaits confirmation, and pops on `disabled`/`configured` (also during a pending transaction: popping returns to uplink DNS). Transient tunnel states and unreadable inputs change nothing. While the stack guard is held after a failed change the reconciler does nothing; the explicit retry is the same `Resume` path as the captive restore. Rules and verify order: invariants §8.

## Local DNS names beside the stack (#239)

Local DNS names (`/api/v1/network/dns-names`, `internal/dhcpserver`) are dhcp `domain` sections, never options of the dnsmasq section. dnsmasq's init script writes each as one `<ip> <names>` line of its per-instance hosts file (`--addn-hosts`, testdata `dhcp/init-dnsmasq-domain-add.txt`), and dnsmasq answers hosts-file names itself before forwarding. So they work under every layer (AdGuard, VPN with `noresolv`, captive) and no layer changes them; rebind protection filters upstream answers only, so a private address is not stripped. They are written through the same `txn.Apply` on `dhcp`, whose config lock serialises them with stack changes; the stack's foreign-staging refusal never sees them because Apply commits in one step. The stack's base and record never contain them. No ADR change to the stack itself: it still owns exactly `server`, `noresolv` and rebind protection.

## DNS over HTTPS is AdGuard's upstream, not a layer (#245)

Decision: DNS over HTTPS is AdGuard Home's `upstream_dns`, carried by the existing `adguard` layer. There is no `doh` layer and no `https-dns-proxy` package.

Why: AGH already speaks DoH (the seed's upstream is `https://dns10.quad9.net/dns-query`) and is the default DNS. A separate `doh` layer would need a precedence slot against `adguard` (one of them would never be effective while both are on), a second fail-open probe and another package. Through AGH, the stack's rules hold unchanged: the `captive` layer outranks `adguard`, so a bypass reaches the hotel's resolvers (encrypted upstreams cannot resolve hotel names, device-facts §7); with the tunnel connected, AGH's DoH queries are router output and follow the kernel route (through wg0 on a full tunnel); and when the DoH server stops answering, AdGuard's liveness probe fails and the `adguard` layer is popped (fail open) like any AGH outage.

Implementation (`internal/adguard/doh.go`, `GET/PUT /api/v1/adguard/doh`): reads `upstream_dns` from AGH's loopback `GET /control/dns_info`; a change is one `https://` URL with a host and no credentials, query or fragment. Before writing, `POST /control/test_upstream_dns` with AGH's own bootstrap resolvers must answer `OK` for it (testdata `adguard/test-upstream-dns-*.json`; the reply is keyed by the normalised URL, so only the values are read); otherwise nothing changes (409 `doh_upstream_unreachable`). The write is `POST /control/dns_config {upstream_dns: [url]}` and is read back. It never touches dnsmasq, the stack's base or its record, and refuses while AGH's install or config guard is held. Trade-off: with AdGuard off, DNS uses the uplink's plain resolvers; DoH without AdGuard is not offered.
