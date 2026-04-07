# Plan: Connection Failover (Ordered Multi-WAN)

**Status:** Not implemented
**Priority:** Medium
**Related requirements:** [2.7 Connection Failover](../requirements/requirements.md#27-connection-failover)

---

## Goal

Provide a solid, OpenWrt-native failover system where users can:

- order available uplinks for failover priority (`Ethernet`, `WiFi/WWAN`, `USB tether`)
- enable or disable each uplink as a failover candidate without breaking the underlying interface config
- configure health tracking targets and thresholds
- see which uplink is currently active and when failover events happened
- receive alerts when the active uplink changes

The feature must use existing OpenWrt mechanisms rather than a custom background reconfiguration loop.

---

## OpenWrt Analysis

### Why `mwan3`

OpenWrt already solves this problem with `mwan3`, which:

- tracks uplink health per interface with `track_ip`, `interval`, `failure_interval`, `recovery_interval`, `down`, `up`, `count`, `timeout`, and `reliability`
- models ordered failover via `member` metrics inside a `policy`
- reacts to `ifup`, `ifdown`, `connected`, and `disconnected` hotplug events
- updates routing and policy rules when an interface tracker changes state

This is a better fit than a custom cron or goroutine solution because it keeps failover logic in the OpenWrt networking layer and avoids us repeatedly mutating `network` UCI state in the background.

### Relevant upstream behavior

The upstream sample config and hotplug scripts confirm:

- interface priority is represented by `member.metric`; lower metric wins
- interface inclusion can be controlled with `config interface '<name>' option enabled '0|1'`
- backup-like behavior is supported with `initial_state 'offline'`, but we should not rely on that for ordinary UI enable/disable semantics
- `mwan3` ignores disabled interfaces during hotplug processing
- `connected` / `disconnected` events recompute active policies without us writing custom routing code

### What we should not do

- Do not build a custom failover daemon that rewrites route metrics every few seconds
- Do not run `wifi`, `wifi up`, or `wifi reload` as part of failover
- Do not blur together "interface is configured/up" and "interface participates in failover"
- Do not install `luci-app-mwan3`; it adds UI weight we do not need because this project already has its own frontend

---

## Product Decisions

### 1. `mwan3` is an optional installable service

Follow the existing service model used for `vnstat`, `SQM`, `Tailscale`, and `AdGuard`:

- add `mwan3` to the Services page as an installable service
- failover configuration lives on the Network page, not on Services
- if `mwan3` is not installed, the Network UI shows a clear install prompt instead of a broken form

This matches the current architecture and keeps package footprint explicit.

### 2. UI "disable" means "exclude from failover policy"

For failover candidates, the UI switch should mean:

- `enabled = true`: interface is tracked by `mwan3` and included in the ordered policy
- `enabled = false`: interface remains configured in OpenWrt, but `mwan3` does not track or route through it for failover

This must be separate from the existing interface up/down controls. A user may want WiFi uplink configured and usable manually, but not eligible for automatic failover.

### 3. Only existing outbound uplinks are candidates

The failover candidate list must be derived from interfaces already known to the app and suitable as uplinks:

- Ethernet WAN interface(s)
- WiFi uplink / WWAN
- USB tether interface

Do not expose:

- LAN
- VPN interfaces
- AP-only WiFi sections

Candidate discovery must be capability-based, not name-based. Labels like `Ethernet WAN`, `WiFi uplink`, and `USB tether` are UI labels only; the backend must discover real candidates from current network/UCI/runtime metadata rather than assuming fixed IDs like `wan`, `wwan`, or `usb`.

### 4. First version is failover-only, not load balancing

The initial policy should be ordered failover, not balancing:

- one active preferred path
- lower-priority interfaces only used when higher-priority ones are offline

This is simpler, aligns with the requirement, and avoids UI complexity around weights.

### 5. Notifications come from observed active-uplink changes

The backend should not rely on parsing LuCI or shell logs for notifications. Instead:

- compute the active uplink from the configured order plus runtime online/tracked state
- emit an alert when that computed active uplink changes

This keeps notifications stable even if `mwan3 status` output format changes.

### 6. Traffic scope is both router-originated and forwarded traffic

Phase 1 should target both:

- router-originated traffic from the device itself
- forwarded client traffic from LAN/AP clients

This matters because users will expect:

- package installs, DNS resolution, NTP, and backend-initiated outbound calls to follow failover
- client traffic to fail over the same way

The generated `mwan3` rule set must therefore be validated on-device against both traffic classes, not only forwarded traffic.

### 7. Failover is immediate; failback is damped

Product behavior for phase 1:

- failover to the next healthy uplink happens immediately when the preferred uplink is tracker-offline
- failback to a higher-priority uplink only happens after a hold-down period of continuous health

Recommended initial hold-down:

- `30s` continuous healthy state before returning to a higher-priority uplink

### 8. Session behavior is explicit

Phase 1 does not promise seamless migration of in-flight sessions across uplink changes.

Expected behavior:

- new connections should use the newly active uplink immediately
- existing long-lived connections may break and reconnect during failover
- device verification must test whether targeted conntrack flushing improves this on the target package version before we promise more

---

## UX Plan

### Placement

Add a new card in `Network > Advanced`:

- title: `Connection Failover`
- follows the existing config-card pattern: read-only summary + `Edit` button
- no always-open complex form

This matches the current UX overhaul rules for configuration cards.

### Read-only state

Show:

- overall status: `Not installed`, `Disabled`, or `Enabled`
- active uplink
- ordered failover list
- health target summary
- last failover event timestamp if available

### Edit state

Show:

- top-level `Enable automatic failover` switch
- orderable list of candidate uplinks
- per-candidate `Use in failover` switch
- optional health target list
- advanced tracker settings in a collapsible area

### Ordering interaction

Use an orderable list with explicit move controls first:

- `Move up`
- `Move down`

This is safer and simpler than starting with drag-and-drop. A drag UI can be added later if the existing component set already supports it cleanly.

### Candidate row content

Each row should show:

- friendly name: `Ethernet WAN`, `WiFi uplink`, `USB tether`
- backing interface name
- current runtime status: `online`, `tracking failure`, `disabled`, `not available`
- include/exclude switch for failover membership

### Validation rules

- at least one candidate must be enabled before overall failover can be enabled
- only one priority number per candidate
- disabled candidates keep their place in the saved order but are skipped by policy generation
- if the currently active uplink is disabled in the editor, show clear confirmation messaging

---

## Backend Design

### New service

Add a dedicated backend service, for example:

- `backend/internal/services/failover_service.go`

Responsibilities:

- discover candidate uplinks from current network/runtime state
- read and write failover config
- generate deterministic `mwan3` UCI config from app config
- report runtime status
- emit failover events into the existing alert pipeline

### Configuration ownership

Use a project-owned config file for UI intent:

- `/etc/travo/failover.json`

This file stores:

- feature enabled flag
- candidate ordering
- candidate membership enabled flags
- health targets
- tracking thresholds

`mwan3` UCI is generated from this source of truth so the app can preserve intent cleanly even when interface names appear or disappear across boots.

### Why not treat `/etc/config/mwan3` as the source of truth

Directly editing and re-reading raw `mwan3` UCI alone makes the UI harder to keep stable because:

- interface names may exist in UCI even when not currently available
- `member` and `policy` names are implementation details, not product-level concepts
- user ordering is clearer in app config than inferred from generated member names

The app should own the product model and render `mwan3` as generated runtime config.

### Proposed API

- `GET /api/v1/network/failover`
- `PUT /api/v1/network/failover`
- `GET /api/v1/network/failover/events`

Suggested response shape:

```json
{
  "available": true,
  "service_installed": true,
  "enabled": true,
  "active_interface": "wan",
  "candidates": [
    {
      "id": "wan",
      "label": "Ethernet WAN",
      "interface_name": "wan",
      "kind": "ethernet",
      "available": true,
      "enabled": true,
      "priority": 1,
      "tracking_state": "online"
    },
    {
      "id": "wwan",
      "label": "WiFi uplink",
      "interface_name": "wwan",
      "kind": "wifi",
      "available": true,
      "enabled": true,
      "priority": 2,
      "tracking_state": "offline"
    }
  ],
  "health": {
    "track_ips": ["1.1.1.1", "8.8.8.8"],
    "reliability": 1,
    "count": 1,
    "timeout": 2,
    "interval": 5,
    "failure_interval": 5,
    "recovery_interval": 5,
    "down": 3,
    "up": 3
  },
  "last_failover_event": {
    "from_interface": "wan",
    "to_interface": "wwan",
    "timestamp": 1774567890000,
    "reason": "tracking_failure"
  }
}
```

### UCI generation strategy

Generate a minimal `mwan3` config:

- one `config interface` section per candidate interface
- one `config member` section per enabled candidate
- one `config policy 'travo_failover'`
- one app-owned default IPv4 rule pointing ordinary traffic to `travo_failover`

The generated policy/rule names must be namespaced to app-owned identifiers only. The failover feature owns only its generated `mwan3` objects and must not assume exclusive ownership of all future `mwan3` rules or policies.

For phase 1, this feature owns the app's default-route failover rule only. Any future custom-routing or policy-routing work must either:

- install higher-specificity rules ahead of the default failover rule, or
- extend the same app-owned rule generation path intentionally

If IPv6 failover is not already robust in this codebase, defer IPv6 policy generation for phase 1 and document that clearly.

Example generated shape:

```uci
config interface 'wan'
	option enabled '1'
	option family 'ipv4'
	list track_ip '1.1.1.1'
	list track_ip '8.8.8.8'
	option reliability '1'
	option count '1'
	option timeout '2'
	option interval '5'
	option failure_interval '5'
	option recovery_interval '5'
	option down '3'
	option up '3'

config member 'wan_p1'
	option interface 'wan'
	option metric '1'
	option weight '1'

config member 'wwan_p2'
	option interface 'wwan'
	option metric '2'
	option weight '1'

config policy 'travo_failover'
	list use_member 'wan_p1'
	list use_member 'wwan_p2'

config rule 'travo_default_v4'
	option dest_ip '0.0.0.0/0'
	option family 'ipv4'
	option use_policy 'travo_failover'
```

### Clean enable/disable behavior

When failover is globally disabled:

- keep `/etc/travo/failover.json`
- keep the `mwan3` package installed if the user installed it
- generate no active app-owned default failover rule
- generate no active app-owned members in the failover policy
- reload `mwan3` so routing falls back to normal non-failover behavior

This removes ambiguity: global disable means "feature off, package still installed, no travo-managed failover policy active".

When an individual candidate is disabled:

- keep the underlying network interface untouched
- regenerate policy omitting that interface from `member` / `policy` generation
- keep a disabled `config interface` section only if needed for stable status reporting; otherwise omit it entirely

### Runtime status collection

Status should come from:

- app config (`failover.json`)
- network runtime status (`ubus network.interface.*`)
- `mwan3` tracker state files or `mwan3 status` command output for per-interface online/offline state

The active interface shown in the UI is a product-facing summary derived from app config plus runtime signals. It is useful, but it is not a substitute for verifying real `mwan3` routing behavior on the device.

The summary can be computed by:

1. taking enabled candidates in ascending priority order
2. selecting the first candidate that is both runtime-up and tracker-online

That same computation should drive:

- UI status badge
- dashboard summary
- alert generation

### Event and alert integration

Add a lightweight watcher owned by `FailoverService`:

- polls or reacts to existing network event refreshes
- recomputes active uplink
- when the chosen active uplink changes, emits an alert through `AlertService`

Example alert text:

- `Connection failover switched from Ethernet WAN to WiFi uplink`

Persist a small in-memory ring buffer of recent failover events first. If historical monitoring is implemented later, this can move to persistent storage.

---

## Frontend Design

### Shared types

Add shared API types and route constants in:

- `shared/src/api/network.ts`
- `shared/src/api/routes.ts`

### Hooks

Add hooks in:

- `frontend/src/hooks/use-network.ts`

Examples:

- `useFailoverConfig()`
- `useSetFailoverConfig()`
- `useFailoverEvents()`

### Components

Expected additions:

- `frontend/src/pages/network/failover-card.tsx`
- optional smaller row/editor subcomponents if needed

Possible dashboard enhancement later:

- extend `frontend/src/pages/dashboard/wan-source-card.tsx` to show `Automatic failover enabled` and active priority source based on failover state instead of heuristic source detection alone

### Styling and existing UI rules

The implementation should follow project conventions:

- use the read-only plus `Edit` card pattern
- keep it in `Network > Advanced`
- avoid explicit ordinary text colors unless needed, and if needed provide light and dark variants
- reuse existing shared primitives and status badges
- do not introduce a permanently noisy expert UI on the main network tab

---

## Implementation Phases

### Phase 0 — Finalize behavior decisions

Before implementation starts, lock these decisions in:

1. traffic scope: both router-originated and forwarded traffic
2. failback behavior: 30s hold-down before preferred uplink takes over again
3. global disable semantics: package installed, no app-owned failover rule active
4. phase 1 transport scope: IPv4 only unless device verification proves IPv6 is low-risk

### Phase 1 — Service registration and backend model

1. Add `mwan3` as an installable service in `ServiceManager`
2. Add shared failover types and route constants
3. Add backend `FailoverService` with:
   - candidate discovery
   - config file load/save
   - generated `mwan3` UCI rendering
   - runtime status read path
4. Add API handlers and OpenAPI coverage

### Phase 2 — UI configuration card

1. Add `Connection Failover` card under `Network > Advanced`
2. Show install prompt when `mwan3` is missing
3. Add read-only summary
4. Add edit mode with:
   - top-level enable switch
   - ordered candidate list
   - per-candidate membership switch
   - health target config
   - advanced tracker settings

### Phase 3 — Runtime apply and validation

1. Generate and write `mwan3` config safely
2. Reload or restart `mwan3`
3. Validate that:
   - at least one enabled member exists when failover is on
   - generated policy is present
   - runtime status can be read back
4. Return hard errors to the UI on invalid or partial apply

### Phase 4 — Alerts and dashboard integration

1. Detect active uplink changes
2. Emit WebSocket alerts
3. Show last failover event in the failover card
4. Optionally enrich the dashboard WAN source card with failover context

### Phase 5 — Device verification and hardening

1. Test on the real OpenWrt target with Ethernet, WiFi uplink, and USB tether where available
2. Verify candidate ordering changes active path as expected
3. Verify disabling a candidate excludes it from failover without deleting its interface config
4. Verify service disable returns routing to ordinary single-WAN behavior

---

## Safety Constraints

### No custom route-flipping daemon

Do not implement a background goroutine that repeatedly rewrites `network` UCI, firewall zones, or route metrics. `mwan3` should own runtime failover once configured.

### Crash-guard rule

Applying failover settings writes live routing policy, so the apply path needs an explicit safety story.

Use the mandatory guard-file pattern for the app-managed apply step:

- `/etc/travo/failover-in-progress`

Apply flow:

1. write `/etc/travo/failover-in-progress`
2. back up the current app-owned `mwan3` sections or generated file content
3. replace only app-owned generated `mwan3` sections/rules, not unrelated `mwan3` content
4. reload `mwan3`
5. verify that:
   - the expected app-owned policy/rule exists
   - at least one enabled candidate is readable at runtime
   - router-originated connectivity still works via a simple outbound health check
6. remove the guard file only after verification succeeds
7. if verification fails, restore the previous generated config, reload `mwan3`, keep the guard file, and surface a hard error

On next startup, if the guard file exists, the backend must skip any automatic failover reconciliation and log a warning until the user explicitly reapplies or redeploys.

### Interface preservation

Applying failover config must never:

- delete saved WiFi uplink profiles
- detach a valid uplink from the existing WAN firewall zone without replacing required behavior
- bring down AP radios
- commit wireless changes that require browser-side confirmation

---

## Testing Strategy

### Unit tests

- config file parsing and validation
- deterministic UCI generation from ordered candidates
- active-interface computation from mixed runtime/tracker states
- candidate discovery mapping capability metadata to UI rows and labels
- alert emission only on real active-uplink transitions

### Integration tests

- API handler tests for `GET` / `PUT`
- service-installed vs not-installed UI states
- validation failures for zero enabled candidates
- disabled candidate omitted from generated policy

### Real-device verification

Test on the OpenWrt device directly:

1. Ethernet primary, WiFi secondary
2. WiFi primary, USB secondary
3. disable highest-priority candidate from UI
4. unplug cable or break upstream path and verify failover
5. restore connectivity and verify preferred interface returns only after the hold-down period
6. verify router-originated traffic still works during failover (`opkg`/`apk`, DNS, NTP, backend outbound checks)
7. verify forwarded client traffic follows the same active uplink

Use device-side checks such as:

- `mwan3 status`
- `ubus call network.interface.<name> status`
- `ip route`
- UI-visible active source and alert notifications

---

## Open Questions

1. Do we want one global health target list for all interfaces first, or per-interface targets later?
2. Should USB tether appear only when currently detected, or also when present in persistent network config but temporarily absent?
3. Can the target device/package combination support IPv6 failover cleanly enough for phase 1, or should we ship IPv4-only first?
4. Should the dashboard WAN source card be upgraded in the same task, or only after the failover card is stable?

---

## Recommended First Implementation Slice

Implement the smallest solid slice first:

1. installable `mwan3` service
2. backend failover config model
3. network advanced card with orderable enabled candidates
4. generated IPv4 failover policy
5. runtime status readback
6. failover alerts

Defer:

- drag-and-drop ordering
- IPv6 failover
- load balancing
- per-interface advanced tracker overrides
- historical event persistence beyond a small recent buffer
