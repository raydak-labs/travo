---
title: "Plan: Connection Failover Gaps"
description: "Gaps and remaining work for Connection Failover feature"
updated: 2026-04-15
tags: [failover, network, plan, gaps]
---

# Plan: Connection Failover Gaps

**Base plan:** `connection-failover.md`
**Status:** Backend/Foundation complete, gaps documented below

---

## Gap 1: Failback Hold-Down

**Missing:** 30-second hold-down before failback to higher-priority interface

### Current Behavior
- `FailoverService.Start()` polls every 10 seconds
- Emits alert immediately when active interface changes
- No delay when returning to preferred interface

### Required Change
Add stateful hold-down tracking:

1. Track "preferred interface online since" timestamp for higher-priority candidates that are currently tracking offline
2. When computing `active_interface`, don't switch to higher-priority interface until it has been healthy for 30 continuous seconds
3. Update `track_states` tracking to include "waiting for hold-down" state

### Implementation
```go
// Add to FailoverService struct
type failoverCandidateState struct {
     onlineSince    time.Time
     holdDownThreshold time.Duration // 30s
}

// In computeActiveInterface:
// 1. Find best enabled+available+tracking-online candidate (current)
// 2. Find all enabled+available candidates now tracking online that are higher priority
// 3. If higher-priority found and has been online >= 30s, switch to it
```

**Estimate:** 2-3 hours (service logic update + tests)

---

## Gap 2: Dashboard WAN Source Card Integration

**Missing:** WAN source card doesn't show failover state

### Current Behavior
- Dashboard shows WAN source via heuristics (network status)
- No indication of failover being active/disabled

### Required Change
Update `wan-source-card.tsx`:

1. Fetch `FailoverConfig` alongside network status
2. When failover enabled, show "Automatic failover" badge
3. Display active uplink name from failover config when available
4. Add link to Network > Advanced > Failover settings

### Design
```html
<div class="flex items-center gap-2">
  <span>WAN Source</span>
  {failoverEnabled && <Badge>Automatic failover</Badge>}
</div>
<div>{failoverActiveInterface || detectedSource}</div>
```

**Estimate:** 1-2 hours (component update + hooks/cache)

---

## Gap 3: Real-Device Verification Documentation

**Missing:** No test procedures documented

### Required Tests
Follow plan Phase 5 verification steps:

1. Ethernet primary, WiFi secondary — verify priority ordering
2. WiFi primary, USB secondary — verify USB detection and tracking
3. Disable highest-priority candidate from UI — verify policy excludes it
4. Unplug cable/break upstream path — verify failover event + alert
5. Restore connectivity — verify hold-down delay before return
6. Router-originated traffic during failover — check `opkg`, DNS, NTP
7. Forwarded client traffic — verify same active uplink

### Device Commands
```bash
# Check failover status
mwan3 status
ip route show
ubus call network.interface.<name> status

# Simulate failure
ip route add prohibit 1.1.1.1
```

**Action:** Create `docs/tests/failover-verification.md` with checklist

**Estimate:** 1 hour (documentation only)

---

## Gap 4: IPv6 Deferral Documentation

**Missing:** No explicit decision record for IPv6 exclusion

### Current State
- Generated rule: `family: 'ipv4'`
- Does not generate IPv6 failover policy/rules

### Required Action
1. Verify target OpenWRT version + mwan3 package supports IPv6 failover
2. Document decision in `docs/architecture.md` under "Networking"
3. Update `connection-failover.md` plan Phase 0 status to "Decision deferred to Phase 2"

**Estimate:** 1 hour (research + docs)

---

## Gap 5: Open Question Resolution

### Global vs Per-Interface Health Targets
**Current:** Global (`health.track_ips` applies to all candidates)
**Decision:** Keep global for Phase 1 (simpler UI, sufficient baseline)
**Future:** Per-interface overrides in Phase 2 if users request

### USB Tether Visibility
**Current:** Shows USB tether from persistent network config even when absent
**Decision:** Keep persistent-based (aligns with config-first philosophy)
**Rationale:** Users can enable/disable failover for USB even when device not connected

---

## Priority Order

1. **Gap 1 (Failback hold-down)** — Critical failover correctness
2. **Gap 5 (Open question doc)** — Quick surface-to-air decision
3. **Gap 3 (Verification docs)** — Enables testing before shipping
4. **Gap 2 (Dashboard integration)** — UX polish, non-blocking
5. **Gap 4 (IPv6 defer)** — Research + documentation, can defer

---

## Remaining Open Questions

1. Should failback hold-down be configurable or fixed at 30s?
2. What alert severity for failover events? (Currently: "warning")
3. How many recent events to keep in ring buffer? (Current: 20)