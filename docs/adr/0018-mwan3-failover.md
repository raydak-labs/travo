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
  owning only `enabled`, `family` and the tracking options below. Adopting is a loud warning.
  An interface section is never deleted; an uplink dropped from the list gets `enabled 0`.
- Tracking (#277, supersedes "liveness from netifd, no `track_ip`"): mwan3 only puts a member
  into a policy once mwan3track reports its interface online. Without `track_ip` no tracker
  runs, `mwan3 interfaces` says "unknown and tracking is disabled" and `travo_failover` never
  gets a member, so every packet takes the main table (seen live on the AXT1800). Every Travo
  interface section therefore carries `track_ip 1.1.1.1` and `8.8.8.8` (two independent anycast
  resolvers, by address so tracking needs no DNS during an outage), `reliability 1` (one reply
  per round is enough: one target filtering ICMP does not flap the uplink), `count 1`,
  `interval 5`, `down 3` (offline after 3 failed rounds, ~15 s; 12 s measured live) and `up 6`
  (online after 6 good rounds, 30 s: the failback hold-down of invariants §9). These are
  constants for now; making them configurable is a later settings change. The owned set grew,
  so the fingerprint schema is `travo-mwan3/2`: sections stamped by a pre-release build read as
  operator-edited once and are overwritten (failover never shipped in a release).
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
- The probe also proves mwan3 routes by the policy (#277), since a request through the main
  table succeeds without mwan3 doing anything: `mwan3 interfaces` reports every chosen uplink
  "online and tracking is active", and `iptables -t mangle -S` shows `mwan3_hook` on PREROUTING
  and OUTPUT, the first `mwan3_rules` jump going to `mwan3_policy_travo_failover`, and that
  chain marking for the first uplink (comment `"<uplink> 1 1"`). Enabling therefore needs every
  chosen uplink online. The probe reads through the `MWAN3Reader` device port.
  Disable stops and disables mwan3 under guard `failover-service-in-progress` and keeps the config.
  It then removes what mwan3 left in `ip`/`ip6 mangle` (see "Failover and Tailscale" below).
  A reconciler (every 5 s, lifecycle worker) keeps mwan3 running exactly while the settings say
  enabled and the committed config is active: it stops mwan3 after a rollback restored the stock
  config, and start/boot-enables it after the user confirmed. It does nothing while a transaction
  awaits confirmation or a failover guard is kept.

## Consequences

- mwan3 2.12.0 drives its state machine through `iptables -t mangle -S` (iptables-nft on fw4).
  It works on 25.12.6 (container: tracking, policy switch and router-local failover with
  masquerade), but iptables-nft refuses to list a table holding native nft rules ("table `mangle'
  is incompatible, use 'nft' tool"). mwan3 then misreads its table as empty and its batches fail
  ("Chain already exists"), at the latest on restart; `mwan3 status` fails. Tailscale's nftables
  mode (the OpenWrt package default) writes native connmark rules into `ip`/`ip6 mangle`. With
  such a rule present the probe fails and the apply rolls back.
  Failover and Tailscale are therefore mutually exclusive (below).

- A rollback of the FIRST apply restores the stock config while mwan3 runs; its catch-all rules
  can be live for up to one reconciler tick (5 s) before mwan3 is stopped.

- An operator's own mwan3 rules placed above Travo's by hand are moved below on the next
  apply. Their sections are not otherwise touched.
- The 30 s failback hold-down of invariants §9 relies on mwan3 hotplug timing; the hardware
  playbook measures it before the invariant is marked `[HW]`.

## Failover and Tailscale (0.2.0, #277)

Verified on the AXT1800: Tailscale runs with `fw_mode nftables` (package default) and writes
native connmark rules (mask `0x00ff0000`) into `ip`/`ip6 mangle`. mwan3 2.12.0 reads its state
only through `iptables -t mangle -S`, which then fails, so it can neither create nor delete its
chains (router log: `create_policies_iptables (travo_failover): iptables-restore: line 2 failed:
Chain already exists`), and its chains stay in `ip mangle` after it stops.

Decision (operator, option b): for 0.2.0 failover and Tailscale are mutually exclusive.

- Enabling failover while the `tailscale` package is installed and tailscaled runs answers 409
  `failover_tailscale_conflict`. Installing Tailscale or bringing it up (login) while the failover
  settings say enabled answers 409 `tailscale_failover_conflict`. Both checks run under one mutex
  shared by the two services, held through the whole failover apply, so neither can start
  between the other's check and its settings write. A refusal writes nothing and keeps no guard.
  A failed presence or settings read is an error, never "absent".
- Disable removes mwan3's leftovers through the `MangleCleaner` device port: it reads
  `nft -j list ruleset ip|ip6`, takes the `mwan3_*` chains of table `mangle` and the rules in
  PREROUTING/OUTPUT/POSTROUTING that jump into them, and deletes them in one `nft -f -`
  transaction (rules, then flush every chain, then delete every chain). Every other chain and
  rule stays; a foreign rule elsewhere that jumps into a `mwan3_` chain refuses the cleanup.
  It is idempotent (nothing to delete, no nft call) and a re-read must find nothing left.
  Container captures: `testdata/container/openwrt-25.12.6/mwan3/nft-ruleset-*.json`.

Why: mwan3 has no nft backend, and teaching it to coexist means changing Tailscale's firewall
mode, which needs its own hardware validation. Refusing the combination is safe and small;
option (a), switching Tailscale to `fw_mode iptables` while failover is on, is tracked in the
follow-up issue (#279).

Limits: a Tailscale installed but stopped does not block failover, yet its init script is
boot-enabled, so it starts again after a reboot and breaks mwan3; stop it for good
(`/etc/init.d/tailscale disable`) or remove it. Tailscale installed outside Travo (LuCI, apk)
is only seen by the failover check.
