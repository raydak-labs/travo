# Connection Failover Verification

**Base plan:** `connection-failover.md` Phase 5
**Status:** Ready for device testing
**Target device:** OpenWRT 23.05+ (GL.iNet Beryl AX, Slate AXT1800)

---

## Prerequisites

- Device: GL.iNet Beryl AX (MT3000) or Slate AXT1800 with OpenWRT 23.05+
- Travo installed and running
- At least 2 uplink interfaces available (Ethernet + WiFi, or WiFi + USB tether)
- SSH access to device at `192.168.1.1`
- mwan3 package installed: `opkg list-installed | grep mwan3`

---

## Test Environment Setup

```bash
# SSH to device
ssh root@192.168.1.1

# Verify mwan3 installed
opkg list-installed mwan3

# Check mwan3 status
mwan3 status

# Enable failover in Travo UI
# Navigate: Network > Advanced > Connection Failover > Enable automatic failover
# Configure at least 2 enabled candidates
```

---

## Test Cases

### Test 1: Priority Ordering with Ethernet Primary, WiFi Secondary

**Objective:** Verify candidate priority controls active uplink selection.

**Setup:**

- Enable Ethernet WAN (priority 1)
- Enable WiFi uplink/wwan (priority 2)
- Both interfaces online and tracking healthy

**Steps:**

1. Configure failover with Ethernet priority 1, WiFi priority 2
2. Apply configuration
3. Check active interface: `mwan3 status | grep "is online"`
4. Verify Ethernet (wan) is active

**Expected Result:**

- Ethernet WAN (wan) shows as active online interface
- WiFi (wwan) shows as online but not active
- Routes prefer Ethernet path

**Verification:**

```bash
# Check active interface
mwan3 status
# Expected: "interface wan is online"
# Expected: "interface wwan is online"

# Check routing priority
ip route show default
# Expected: Default route via wan interface
```

---

### Test 2: WiFi Primary, USB Secondary with USB Detection

**Objective:** Verify USB tether detection and tracking.

**Setup:**

- Enable WiFi uplink (priority 1)
- Enable USB tether (priority 2)
- Connect USB tethering device

**Steps:**

1. Connect USB tethering device to router
2. Wait 30 seconds for device detection
3. Configure failover with WiFi priority 1, USB priority 2
4. Apply configuration
5. Check `mwan3 interfaces` output

**Expected Result:**

- USB tether interface detected and tracked
- WiFi or USB active depending on health (WiFi if both healthy)
- USB shows in failover candidate list

**Verification:**

```bash
# USB interface detection
ls /sys/class/net/ | grep usb

# Failover candidate list
# Navigate: Network > Advanced > Connection Failover
# Verify USB tether appears in candidate list
```

---

### Test 3: Disable Highest-Priority Candidate from UI

**Objective:** Verify UI enable/disable excludes candidate from failover policy.

**Setup:**

- Enable Ethernet WAN (priority 1) and WiFi uplink (priority 2)
- Both interfaces online and tracking healthy

**Steps:**

1. Verify Ethernet is active
2. In Travo UI, disable Ethernet WAN from failover candidates
3. Apply configuration
4. Check active interface changes to WiFi

**Expected Result:**

- Ethernet removed from failover policy
- WiFi becomes active interface
- Ethernet network interface remains configured/useable
- No disruption to network configuration beyond failover

**Verification:**

```bash
# Check mwan3 policy members
uci show mwan3 | grep use_member
# Expected: Only wwan member in travo_failover policy

# Check active interface
mwan3 interfaces
# Expected: wwan active, wan not tracking (disabled)

# Verify Ethernet still configured
ubus call network.interface.wan status
# Expected: Interface exists, up, but not in failover
```

---

### Test 4: Simulate Uplink Failure with Failover Event

**Objective:** Verify automatic failover when uplink becomes unhealthy.

**Setup:**

- Ethernet WAN (priority 1) active and healthy
- WiFi uplink (priority 2) enabled and healthy

**Steps:**

1. Verify Ethernet active
2. Simulate Ethernet failure:

   ```bash
   # Block primary health check IP
   ip route add prohibit 1.1.1.1
   ```

3. Wait 30-60 seconds for mwan3 detection
4. Check failover event in Travo UI
5. Verify WiFi becomes active

**Expected Result:**
-Ethernet tracking state changes to offline

- Automatic failover to WiFi uplink
- Failover event logged in Travo alerts
- New connections use WiFi path

**Verification:**

```bash
# Check health tracking state
mwan3 interfaces
# Expected: "interface wan is offline"
# Expected: "interface wwan is online"

# Check routing
ip route show default
# Expected: Default route via wwan interface

# Check Travo alerts
# Navigate: Dashboard
# Verify failover alert displayed

# Restore connectivity
ip route del prohibit 1.1.1.1
```

---

### Test 5: Verify 30-Second Hold-Down Before Failback

**Objective:** Verify hold-down period prevents immediate return to higher-priority interface.

**Setup:**

- Ethernet WAN (priority 1) failed/offline
- WiFi uplink (priority 2) active and stable

**Steps:**

1. Block Ethernet health check (Test 4 setup)
2. Verify WiFi is active
3. Restore Ethernet connectivity immediately:

   ```bash
   ip route del prohibit 1.1.1.1
   ```

4. Wait 15-20 seconds
5. Check active interface (should still be WiFi)
6. Wait another 15+ seconds (>30s total)
7. Check active interface (should now be Ethernet)

**Expected Result:**

- WiFi remains active for 30 seconds after Ethernet recovers
- Ethernet shows tracking online after 15s
- Failover to Ethernet occurs only after 30s hold-down
- No rapid switching between interfaces

**Verification:**

```bash
# Monitor tracking state
watch -n 5 'mwan3 interfaces | grep -E "wan|wwan"'
# Timeline:
# 0-15s: wan offline, wwan online
# 15-30s: wan online, wwan online (hold-down active)
# 30s+: wan online, wwan online (wan switches back to active)

# Check Travo events
# Navigate: Network > Advanced > Connection Failover > Events
# Verify event timestamps match hold-down behavior
```

**Notes:**

- Hold-down is critical to prevent interface flapping
- Measured from when tracking state becomes "online"
- Configured as constant: 30 seconds in failover_service.go

---

### Test 6: Router-Originated Traffic During Failover

**Objective:** Verify router commands use active failover interface.

**Setup:**

- Ethernet WAN (priority 1) active
- WiFi uplink (priority 2) enabled
- Trigger failover to WiFi (Test 4)

**Steps:**

1. Block Ethernet, force WiFi failover
2. Run router-originated traffic tests:

   ```bash
   # Package manager
   opkg update

   # DNS resolution
   nslookup example.com

   # NTP sync
   ntpd -q -p pool.ntp.org

   # Outbound connectivity
   curl -I https://api.github.com
   ```

**Expected Result:**

- All commands succeed using WiFi uplink
- No errors indicating network unreachable
- DNS resolution uses WiFi path
- App updates/installs work via WiFi

**Verification:**

```bash
# Check which interface used for DNS
grep nameserver /etc/resolv.conf

# Check active routes during commands
ip route get 1.1.1.1
# Expected: Route via wwan interface

# Verify no routing errors in logs
logread | grep -E "network|route" | tail -20
```

---

### Test 7: Forwarded Client Traffic During Failover

**Objective:** Verify client traffic follows same active uplink.

**Setup:**

- Ethernet WAN (priority 1) active
- WiFi uplink (priority 2) enabled
- Client device connected via LAN or WiFi AP

**Steps:**

1. Block Ethernet, force WiFi failover
2. On client device, run:

   ```bash
   # External IP check
   curl ipinfo.io/ip

   # Traceroute
   traceroute 1.1.1.1

   # Continuous ping
   ping -c 10 1.1.1.1
   ```

**Expected Result:**

- Client external IP matches WiFi uplink
- Traffic routes through WiFi interface
- No packet loss or route changes during test
- Failover transparent to client (except IP change)

**Verification:**

```bash
# Check conntrack for active connections
conntrack -L | grep <client-ip>

# Verify NAT rules
iptables -t nat -L -v -n | grep -E "wan|wwan"

# Check traffic counters
ifconfig wwan  # Check TX/RX counters for client traffic

# On router, trace client packet path
tcpdump -i wwan -n host <client-ip>
# Expected: Client traffic visible on wwan interface
```

---

## Troubleshooting

### Issue: Failover not switching interfaces

**Check:**

```bash
# mwan3 service status
/etc/init.d/mwan3 status

# Config syntax
mwan3 check
# Or validate UCI
uci show mwan3

# Kernel modules
lsmod | grep mwan
```

**Fix:**

```bash
# Reload mwan3
/etc/init.d/mwan3 reload

# Restart if needed
/etc/init.d/mwan3 restart

# Check logs
logread | grep mwan3 | tail -50
```

---

### Issue: Health checks failing immediately

**Check:**

```bash
# Test health check IPs from device
ping -c 3 1.1.1.1
ping -c 3 8.8.8.8

# Check DNS resolution
nslookup 1.1.1.1

# Verify tracking settings
uci show mwan3 | grep -E "track_ip|interval|timeout"
```

**Fix:**

```bash
# Adjust health check settings in Travo UI
# Increase timeout or interval
# Try different health check IPs (8.8.4.4, 1.0.0.1)
```

---

### Issue: Hold-down not working (immediate failback)

**Check:**

```bash
# Check Travo logs for failover service
logread | grep -E "failover|online|hold" | tail -50

# Verify service running
pgrep -af failover_service

# Check mwan3 tracking timing
mwan3 status
```

**Fix:**

```bash
#Restart failover service
/etc/init.d/travo restart  # Or equivalent

# Verify service logs
# Expected: onlineSince timestamps being tracked
```

---

## Cleanup and Reset

After testing, restore normal operation:

```bash
# Remove any blocking rules
ip route flush prohibit

# Verify all interfaces healthy
mwan3 status

# Check default routes
ip route show default

# Verify failover settings as expected
# Navigate: Network > Advanced > Connection Failover
```

---

## Success Criteria

**ALL tests pass:**

- ✅ Priority ordering respected
- ✅ USB tether detection works
- ✅ UI disable excludes from policy
- ✅ Automatic failover on failure
- ✅ Hold-down period observed
- ✅ Router-originated traffic works
- ✅ Client traffic forwards correctly

**No critical errors:**

- ✅ No mwan3 service crashes
- ✅ No routing loops
- ✅ No config corruption
- ✅ No持续的接口翻跳

**Verification complete when:**

- All manual test steps executed successfully
- Evidence collected for each test case (commands, outputs)
- Any issues documented with troubleshooting steps
- Test environment restored to clean state

---

## Next Steps After Verification

1. **Document issues found** in test report
2. **Update plan** if behavior differs from expectations
3. **Address critical bugs** before shipping
4. **Create user guide** for failover configuration
5. **Finalize Gap 4 (IPv6 deferral)** with research findings
6. **Prepare for production deployment** on target devices
