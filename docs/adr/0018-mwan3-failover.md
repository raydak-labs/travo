# 0018 Multi-WAN failover with mwan3

Status: accepted

## Context

Invariants §9 and device-facts §9 fix the rules for failover (Travo's JSON is the source of
truth, `travo_` sections, provenance ownership, no delete of foreign sections, staged apply,
IPv4 only). The mwan3 2.12.0 package of the 25.12.6 feed was inspected without installing it
(`testdata/device/axt1800-25.12.6/mwan3/`):

- Its `/etc/config/mwan3` ships interfaces `wan` (enabled, four `track_ip`), `wan6`, `wanb`,
  `wanb6` (disabled), ten members, five policies and three catch-all rules `https`,
  `default_rule_v4`, `default_rule_v6` into policy `balanced`. With no healthy member this
  policy is `unreachable`: the outage of device-facts §9.
- mwan3 keys `config interface` sections by the netifd interface NAME. A `travo_wan`
  interface section would track nothing.
- mwan3 evaluates rules in config order; the first match wins.
- Its init script reloads on `procd_add_reload_trigger 'mwan3'`.

## Decision

- Generated sections: member `travo_m_<uplink>` (metric = priority, weight 1), policy
  `travo_failover` (`last_resort default`: with every member down the main table is used, not
  `unreachable`), rule `travo_v4` (0.0.0.0/0, ipv4 → travo_failover), rule `travo_v6`
  (::/0, ipv6 → `default`, IPv6 stays on the main table; no IPv6 policy, the deliberate
  ceiling). Both rules are moved to positions 0 and 1 with `uci reorder`, so no stock rule
  matches first. Stock sections are never deleted or edited.
- Interface sections are the one exception to the `travo_` prefix, forced by mwan3: Travo
  creates `wan`/`wwan`/`usbtether` when missing and ADOPTS an existing one (the stock `wan`),
  owning only `enabled`, `family` and `track_ip` (removed: liveness from netifd ifup/ifdown,
  not ping). Adopting is a loud warning. An interface section is never deleted; an uplink
  dropped from the list gets `enabled 0`.
- Provenance: every written section carries `travo_fp`, a hash of its name, type and owned
  values under a schema tag (not the build version, so an upgrade does not orphan sections).
  Owned = stamp present and still matching. A stamped section whose owned values were changed
  by hand is overwritten with a loud `operator_edited` warning; a stale one is deleted only
  while its stamp matches, otherwise kept with a `stale_edited` warning.
- Writes are set-if-different, so re-applying unchanged settings stages nothing.
- `luci-app-mwan3` is not needed: Travo stages with the uci CLI under its own locks and the
  transaction watchdog (ADR 0005), not rpcd `uci apply`, so the missing-ACL failure of
  device-facts §9 does not apply.
- Failback hold-down and the priority walk are mwan3's own: members in metric order, the
  lowest metric that is online wins; an interface comes back online on netifd `ifup`.

- Lifecycle (#209): `mwan3` is installed through the `PackageManager` port under guard
  `failover-install-in-progress` and immediately stopped and disabled, so its stock config never
  runs. Enable (needs a real outbound probe online first) stages the plan in ONE `mwan3`
  transaction; the probe restarts mwan3 and proves every managed section reads back stamped, the
  Travo rules on top, mwan3 running and the outbound probe online, then the user confirms.
  Disable stops and disables mwan3 under guard `failover-service-in-progress` and keeps the config.
  A reconciler (every 5 s, lifecycle worker) keeps mwan3 running exactly while the settings say
  enabled and the committed config is active: it stops mwan3 after a rollback restored the stock
  config, and start/boot-enables it after the user confirmed. It does nothing while a transaction
  awaits confirmation or a failover guard is kept.

## Consequences

- A rollback of the FIRST apply restores the stock config while mwan3 runs; its catch-all rules
  can be live for up to one reconciler tick (5 s) before mwan3 is stopped.

- An operator's own mwan3 rules placed above Travo's by hand are moved below on the next
  apply. Their sections are not otherwise touched.
- The 30 s failback hold-down of invariants §9 relies on mwan3 hotplug timing; the hardware
  playbook measures it before the invariant is marked `[HW]`.
