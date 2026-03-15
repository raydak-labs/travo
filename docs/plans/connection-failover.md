# Plan: Connection Failover (Multi-WAN)

**Status:** Not implemented
**Priority:** Low (🔮 future)
**Related requirements:** [2.7 Connection Failover](../requirements/requirements.md#27-connection-failover)

---

## Goal

Automatically switch between WAN sources (Ethernet > WiFi > USB Tether) based on connectivity health, with configurable priorities and health checks.

---

## Background

OpenWRT supports multi-WAN via the `mwan3` package, which provides:
- Multiple WAN interface tracking
- Health checks (ping-based)
- Policy-based routing
- Failover and load balancing

**Alternative:** Custom lightweight implementation using metric-based routing + cron health checks. `mwan3` is more robust but adds ~100KB and complexity.

---

## Approach: mwan3-based (Recommended)

### Phase 1 — mwan3 as Optional Service

1. Add `mwan3` + `luci-app-mwan3` to service registry
2. Install via Services page
3. Auto-configure for detected WAN interfaces

### Phase 2 — Auto-Configuration

After install, set up default failover policy:

**Interface tracking:**
```
mwan3 interface wan  → track via ping to 8.8.8.8, interval=10s, failure=3, recovery=3
mwan3 interface wwan → track via ping to 8.8.4.4, interval=10s, failure=3, recovery=3
```

**Member + policy:**
```
wan:  metric=1 (highest priority)
wwan: metric=2
usb:  metric=3
```

**Default rule:** All traffic → failover policy (try wan, then wwan, then usb)

### Phase 3 — Configuration UI

**API:**
- `GET /api/v1/network/failover` — current failover config & status
- `PUT /api/v1/network/failover` — update priorities, health check targets

**Frontend:**
- Drag-and-drop priority ordering of WAN sources
- Health check configuration (ping target, interval, failure threshold)
- Current status: which WAN is active, last failover event

### Phase 4 — Dashboard Integration

- Show failover status on dashboard
- Alert via WebSocket on failover event ("Switched from Ethernet to WiFi at 14:30")
- Timeline of failover events

---

## Alternative: Lightweight Custom Implementation

If mwan3 is too heavy, a simpler approach:

1. **Cron job** (every 30s): Ping health check target through each WAN interface
2. **On failure:** Adjust interface metrics via `uci set network.<iface>.metric`
3. **On recovery:** Restore original metrics
4. **Notification:** Write event to alert service

This is simpler but less robust (no policy routing, no load balancing).

---

## Notes

- Failover interacts with VPN: if VPN is on WAN and WAN fails, VPN should reconnect on new WAN
- mwan3 handles this automatically if configured correctly
- Test thoroughly: failover during active connections should be seamless
- Consider user notification: "Internet via WiFi (Ethernet failed at 14:30)"
