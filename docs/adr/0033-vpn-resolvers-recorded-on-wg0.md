# 0033 The tunnel transaction records the applied resolvers on wg0

Status: accepted (#289); changes the resolver source of the `vpn` layer in [0010](0010-dns-layer-stack.md)

## Context

The `vpn` layer pushed the SELECTED profile's `DNS =` resolvers. Selecting another profile without re-applying switched DNS to its resolvers while traffic still used the old tunnel. `wg0` carries no DNS, so the applied profile cannot be derived from it: matching stored profiles against `wg0` raced with staging and could not tell apart profiles that differ only in DNS (#295 review).

## Decision

- Enable stages the applied profile's resolvers as `network.wg0.travo_dns` (space-separated, or `none`) in the WireGuard transaction, so rollback restores them with `wg0`. If `wg0` already runs as the profile, only that option is staged, without a restart or handshake proof.
- The reconciler reads them under the `network` write lock (`txn.Manager.ShowSettled`), and pushes only when no transaction began (a begin counter), and none held its guard, between the start of the read and the push itself. The tunnel state is read last. So it never acts on a staged delta or on an apply that is unconfirmed or was rolled back. An unreadable value changes nothing, except that a tunnel that is off still pops.
- A `wg0` without the option (applied before #289) uses the selected profile until the next Enable records it.
- Split routes use the same resolvers; when Enable changes them, the resolver `/32`s move with them and `wg0` restarts (#296). Every split route Travo writes carries `travo_resolver` (`1` for a resolver `/32`, `0` for a user prefix), so a user `/32` equal to a resolver stays put. Route sections written before the marker treat an unmarked `/32` equal to a recorded resolver as automatic.

## Consequences

The tunnel transaction writes DNS metadata but still never touches dnsmasq. netifd ignores both options: in a booted `aarch64_generic-25.12.6` container `wg0` came up with it and kept its uptime and ifindex when only the option changed (`reload_config`); a route section with `travo_resolver` is installed as usual. The hardware check is in the "VPN DNS layer" playbook step.
