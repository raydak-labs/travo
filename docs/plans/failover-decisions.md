---
title: "Decisions: Connection Failover"
description: "Resolved design decisions for connection failover system"
updated: 2026-04-15
tags: [failover, network, decisions, traceability]
---

# Connection Failover: Resolved Design Decisions

This document captures finalized decisions for the multi-WAN failover feature implemented in Phase 0.

---

## Global Health Check Targets

**Decision:** Use `1.1.1.1` (Cloudflare DNS) and `8.8.8.8` (Google DNS) as default upstream targets.

**Rationale:**
- Public DNS servers have strong availability globally.
- Both support ICMP and TCP/UDP on port 53, giving mwan3 multiple tracking methods.
- Geographically distributed service reduces risk of regional outage.
- No local dependencies that could themselves fail.

**Phase 1 assumed behavior:**
- Targets are global (not region- or ISP-specific).
- Configurable via UI, but defaults remain fixed across deployments.
- mwan3 track configuration uses `track_ip` list with default `interval: 5`, `timeout: 2`.

## USB Tether Visibility from Persistent Config

**Decision:** USB tether interfaces are discovered from UCI `network` config sections, not from runtime hotplug state alone.

**Rationale:**
- Persistent config survives reboots and allows ordering priorities in the failover UI.
- Runtime-only discovery would require user to manually add interfaces each boot.
- `network` config with `device` matching known USB device names (`usb0`, `usb1`, `eth1`, `eth2`) provides reliable detection.

**Implementation:**
- `FailoverService.addDiscoveredUSBNetworkCandidates()` scans UCI for `usb_tether` section and `device` values matching USB patterns.
- Candidates are preserved in `/etc/travo/failover.json` with user-configured priority and enabled state.

## Fixed 30-Second Hold-Down Duration

**Decision:** Failback hold-down is fixed at 30 seconds and is not user-configurable in Phase 1.

**Rationale:**
- 30 seconds is sufficient to detect slow-moving flaps ( intermittent WiFi recovery, handshaking USB modems) without being overly cautious.
- Adding configurability adds UI complexity and requires validation.
- Fixed duration simplifies observability and troubleshooting (no per-deployment variance to account for).
- If deployments prove 30 seconds too short or too long, it can be made configurable in Phase 2.

**Implementation:**
- Backend constant `failbackHoldDown = 30 * time.Second`.
- `FailoverService.computeActiveInterface()` tracks `candidateOnlineSince` timestamps and enforces threshold before selecting a higher-priority interface.

## Deferred Items

### IPv6 Support

**Decision:** Phase 1 is IPv4-only. IPv6 failover is deferred to Phase 2.

**Rationale:**
- mwan3 supports IPv6 via separate policy and rule stanzas with `family: "ipv6"`, which doubles configuration surface.
- IPv6 reachability and routing behavior differs significantly (SLAAC, RFC 4941 privacy addresses, ISP-native vs tunnel addressing).
- Use cases for IPv6-only or IPv6-preferring failover are unclear; most users care about IPv4 connectivity first.
- Priority is to get a working failover path for common travel-router use (WiFi, Ethernet, USB tether).

### Configurable Alerts

**Decision:** Basic alerting (notification channel, broadcast) is implemented, but per-interface thresholds and retry counts are not user-configurable in Phase 1.

**Rationale:**
- Alert system already supports topic-based filtering (`connection_failover`).
- mwan3's internal `up`, `down`, `reliability`, `count` parameters provide sufficient tuning for most cases.
- Reducing alert noise is achieved via hold-down preventing rapid failover events in the first place.
- Per-interface thresholds would require additional UI fields and validation logic.

### Advanced Routing Rules

**Decision:** Phase 1 uses a single `travo_default_v4` rule matching `dest_ip: 0.0.0.0/0`. Per-destination or per-protocol routing is deferred.

**Rationale:**
- Most users want "all traffic goes through the best available link".
- More complex policies (some traffic always on VPN, some always on WiFi) require UI for rule construction.
- Single rule keeps implementation and testing surface manageable.

---

**Reference:** This document captures decisions documented in [`docs/plans/connection-failover.md`](./connection-failover.md) and [`docs/architecture.md#51-multi-wan-failover-configuration`](../architecture.md#51-multi-wan-failover-configuration).