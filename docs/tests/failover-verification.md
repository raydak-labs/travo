---
title: "Test Plan: Connection Failover Verification"
description: "End-to-end test scenarios for failover system"
updated: 2026-04-15
tags: [failover, testing, verification, network]
---

# Connection Failover Verification Test Plan

This document lists manual and automated test scenarios to verify the failover system works correctly across edge cases and real-world conditions.

---

## Test Environment

- Target device: OpenWrt router with at least two uplinks (Ethernet and WiFi, or WiFi and USB tether).
- Test tools: Ping (`ping -I <interface>`), curl, browser dashboard, SSH access to view logs.

---

## Test Scenarios

### SCENARIO 1: Immediate Failover on Primary Failure

**Setup:**
- Two configured candidates: `wan` (priority 1), `wwan` (priority 2).
- Both interfaces online and tracked.
- Failover enabled.

**Steps:**
1. Via SSH, disconnect primary interface (e.g., `ifconfig wan down` or unplug Ethernet).
2. Wait up to 20 seconds for mwan3 to detect track failures and switch policy.
3. Via dashboard, verify `active_interface` changes to `wwan`.
4. Verify ping from LAN host works via `wwan`. Use `ping -I wwan 1.1.1.1`.

**Expected:**
- Failover event is logged.
- Traffic routes via `wwan`.
- No UCI config changes other than guard file write/remove (mwan3 handles routing).

---

### SCENARIO 2: Hold-Down Prevents Rapid Failback

**Setup:**
- `wan` (priority 1) is offline (e.g., Ethernet unplugged).
- `wwan` (priority 2) is stable, active.
- Failover enabled.

**Steps:**
1. Verify dashboard shows `active_interface: wwan`.
2. Connect `wan` (plug Ethernet or bring interface up).
3. Immediately check dashboard; it should still show `wwan`.
4. Wait 20 seconds; dashboard should still show `wwan`.
5. Wait 10 more seconds; dashboard should switch to `wan`.

**Expected:**
- No failback occurs until 30 seconds after `wan` comes online.
- Avoids flapping if primary connection is unstable during recovery.

---

### SCENARIO 3: Multiple Candidates Ordered Correctly

**Setup:**
- Three candidates: `wan` (priority 1), `wwan` (priority 2), `usb0` (priority 3).
- All online and tracked.

**Steps:**
1. Verify dashboard lists interfaces in order: wan, wwan, usb0.
2. Disconnect `wan`.
3. Verify failover to `wwan`.
4. Disconnect `wwan`.
5. Verify failover to `usb0`.
6. Reconnect `wan`, wait 30s.
7. Verify failback to `wan` (skipping `wwan` since it's still offline).

**Expected:**
- Priority ordering reflects user-configured sequence.
- Failover traverses candidates in priority order, not just "first available".

---

### SCENARIO 4: Disabled Candidate Not Considered

**Setup:**
- `wan` (priority 1) is enabled and online.
- `wwan` (priority 2) is disabled but online.

**Steps:**
1. Verify `active_interface: wan`.
2. Disconnect `wan`.
3. Wait 20 seconds; verify no failover occurs (active becomes empty).
4. Enable `wwan` via dashboard.
5. Verify immediate failover to `wwan`.

**Expected:**
- Disabled candidates are excluded from failover even if online.
- Dedicated UI checkbox governs inclusion.

---

### SCENARIO 5: Failover When Service Not Installed

**Setup:**
- mwan3 package not installed on target device.

**Steps:**
1. Open dashboard Network > Advanced view.
2. Verify failover card shows "Not installed" status.
3. Attempt to enable failover button should be disabled or error with clear message.

**Expected:**
- UI reflects system state (service_installed: false).
- Clear引导用户 to install mwan3 from Services page.

---

### SCENARIO 6: Service Guard File Prevents Reapplication

**Setup:**
- Failover is enabled with active config.
- Guard file exists at `/etc/travo/failover-in-progress` (simulate crashed apply).

**Steps:**
1. Restart backend to trigger service reload.
2. Via dashboard, check failover status; it should remain as-is (no config rewrites).
3. Verify guard file still exists.
4. Remove guard file manually via SSH: `rm /etc/travo/failover-in-progress`.
5. Trigger config reload or apply a new failover setting.
6. Verify guard file is recreated briefly during apply and removed after success.

**Expected:**
- Guard file prevents mutation on service reload.
- Manual removal allows reapplication.
- Guard file lifecycle matches crash guard protocol.

---

### SCENARIO 7: Dashboard Accurately Reports Active Interface

**Setup:**
- Multiple uplinks configured.

**Steps:**
1. Authenticate to dashboard (web UI).
2. Note `WAN Source` card on dashboard and failover status in Network > Advanced.
3. Via SSH, force uplink switch by disabling currently active interface.
4. Refresh dashboard; verify card updates to reflect new active interface.
5. Switch back; verify dashboard updates again.

**Expected:**
- Dashboard reflects backend's computed `active_interface` from `/api/v1/network/failover`.
- Updates happen reasonably quickly (default 10-second heartbeat).
- "Automatic failover" badge appears when enabled.

---

## Test Cleanup

After running scenarios, restore device to a clean state:
- Re-enable all interfaces that were disabled for testing.
- Verify failover配置回到预期状态 (candidates in desired order).
- Remove any temporary guard files created during manual testing.
- Confirm no lingering UCI changes beyond managed sections (`travo_*`).

---

**Reference:** These tests correspond to behavior described in [`docs/plans/connection-failover.md`](./connection-failover.md) and [`docs/plans/failover-decisions.md`](./failover-decisions.md).