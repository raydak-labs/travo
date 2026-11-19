# 0021 AdGuard Home on :53 as an opt-in expert topology

Status: accepted (#186); implemented, hardware run pending (docs/testing.md "AdGuard Home on port 53")

[0006](0006-adguard-package-and-topology.md) keeps dnsmasq on :53 and forwards to AGH on `127.0.0.1:5353`. AGH then sees every query from 127.0.0.1 (a deliberate ceiling in invariants). Operators who want per-client statistics and per-client rules need AGH to see the client address. This ADR fixes how that works without giving up the single DNS layer stack of [0010](0010-dns-layer-stack.md).

## Options considered

1. **dnsmasq adds the client to the query** (`add-subnet=32` / `add-mac`). No port changes, but per AdGuard Home 0.107 documentation clients are identified by the packet source (and DoH/DoT client IDs), not by ECS or the MAC option, so the statistics would stay at 127.0.0.1. Rejected; check on hardware before revisiting.
2. **AGH replaces dnsmasq for DNS** (dnsmasq `port=0`). AGH would need its own upstreams for captive, VPN and Tailscale resolvers, which brings back per-feature DNS state outside the stack, exactly what 0010 removed. Rejected.
3. **AGH in front, dnsmasq behind (chosen).** AGH listens on the lan address and 127.0.0.1 port 53 and forwards everything to dnsmasq on `127.0.0.1:5354`; dnsmasq keeps DHCP names, rebind protection and the layer stack (captive, vpn, tailscale) and reaches the upstreams. Only the bottom hop changes; every layer keeps working because it still edits dnsmasq.

## Design

- **Mode** `default | expert`, persisted in UCI `travo-greenfield.main.adguard_topology`. Default stays `default`. Expert is offered only with AGH installed, running and loopback-migrated.
- **Switch to expert** is one guarded change under `adguard-topology-in-progress`: (a) `dhcp` through the transaction module (snapshot, confirm window): `dhcp.@dnsmasq[0].port=5354` (the router itself keeps resolving through 127.0.0.1:53, now AGH) and `dhcp.lan.dhcp_option` gains `6,<lan ip>` (dnsmasq stops advertising itself as DNS once it is off port 53; the guest network gets the same for its own address); (b) the AGH YAML swap of 0006 sets `dns.bind_hosts` to `127.0.0.1` and the lan address, `dns.port: 53`, `upstream_dns: [127.0.0.1:5354]`, `private_networks`/`use_private_ptr_resolvers` to dnsmasq. Order: dnsmasq moves first, then AGH binds :53, then a probe resolves a fresh name through `<lan ip>:53` and `127.0.0.1:53`; only then the user can confirm. Failure or no confirm restores both (dnsmasq first back to :53).
- **The `adguard` layer is not pushed in expert mode**: dnsmasq must forward to the real upstreams, otherwise AGH → dnsmasq → AGH loops. Switching to expert pops it first; switching back re-runs the default-DNS logic of 0006.
- **Liveness and fail-open**: the probe resolves through `127.0.0.1:53`. After M failures the worker reverts to `default` (dnsmasq back on :53, DHCP option removed, AGH back to `127.0.0.1:5353`), persists `fail_open_since`, shows the banner; re-entering expert needs the user. While AGH is down in expert mode nothing answers on :53, so the revert must not wait for a user: it is the one automatic DNS port change, guarded, with its own retry on the next boot via `Recover`.
- **Guest and VPN**: the kill switch and VPN DNS are dnsmasq-side and unchanged. Guest clients query the guest address on :53, so AGH binds the guest address too when the guest AP exists; the guest transaction re-applies the binding when it creates or removes the guest network.
- **Invariants change** (§8): "dnsmasq on :53 is the single DNS authority" becomes "dnsmasq is the single DNS authority for upstream choice; in expert mode AGH owns :53 and forwards only to dnsmasq". "AdGuard binds loopback only" becomes "loopback, plus the lan/guest addresses on port 53 in expert mode; the admin UI stays loopback".
- **API/UI**: `PUT /api/v1/adguard/topology {mode}` returns a transaction to confirm like the WiFi flows; the Services page offers it under an "Expert" disclosure with the warning that an AGH outage is a short DNS outage until the automatic revert.

## Why not now

The design changes two invariants and the lockout surface of DNS (a failed switch is a LAN-wide DNS outage), and it needs a hardware run with a phone and a laptop to prove the revert. It is implemented as its own change set after 0.2's other DNS work (Tailscale layer) lands, so both are proven together on the router.

## Implementation (#186)

What the code does where the design above left room, and where it differs:

- **Both directions are a transaction.** `PUT /api/v1/adguard/topology {mode}` returns a pending transaction for `expert` and for `default`: one code path, and a failed return to dnsmasq on :53 restores expert exactly. Configs: `dhcp` and `travo-greenfield` (the mode), so the watchdog restores the mode together with dnsmasq's port.
- **Restores put AGH first.** AGH holding `127.0.0.1:53` can stop dnsmasq from binding :53, so every restore restarts AGH with its kept YAML first and then restarts dnsmasq (the design said "dnsmasq first"). The switch's probe runs after dhcp is committed: for expert it waits until dnsmasq answers on `127.0.0.1:5354`, then swaps AGH's YAML, restarts AGH and resolves a fresh name on port 53 of every bound address; for default it swaps AGH back, restarts dnsmasq and resolves on 127.0.0.1 and the lan address.
- **The outcome is read from UCI.** The daemon keeps AGH's old YAML and the `adguard-topology-in-progress` guard until the transaction is no longer pending; the liveness worker (every 15 s) and startup then compare the committed mode with the target: equal means confirmed, different means the watchdog restored dhcp, and AGH's YAML is put back. After a missed confirm DNS can be off for up to one tick plus AGH's restart.
- **dhcp edits**: `@dnsmasq[0].port=5354`; `dhcp_option 6,<address>` on every served pool (lan, guest when it exists); `dhcp.lan.dns_service=0`, so odhcpd stops announcing the lan's IPv6 address as resolver (nothing answers DNS there in expert mode). The plan keeps every option's previous value; its inverse is the way back.
- **AGH YAML edits** (line-exact, like the 0006 migration): `bind_hosts`, `port: 53`, `upstream_dns: [127.0.0.1:5354]`, `fallback_dns: []` (a fallback would bypass the layer stack), `local_ptr_upstreams: [127.0.0.1:5354]`, `use_private_ptr_resolvers: true`. The replaced lines are kept, so switching back restores exactly them and keeps every other edit made in AGH's UI meanwhile.
- **Refused in expert mode** (409 `adguard_expert_topology`): stopping AGH, the "default DNS" toggle, changing the DoH upstream (AGH's upstream must stay dnsmasq), and creating the guest network (its address would not be bound; switch back first). Changing the lan address answers 409 `lan_adguard_expert` for the same reason (ADR 0028). The loopback migration never runs on the expert config.
- **Known gap**: changing the lan address in expert mode leaves AGH bound to the old address; it stops answering and the automatic revert returns to default.
