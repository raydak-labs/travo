---
title: "Plan: USB Tethering Support"
description: "Planning / design notes: Plan: USB Tethering Support"
updated: 2026-04-13
tags: [plan, traceability, usb]
---

# Plan: USB Tethering Support

**Status:** Not implemented
**Priority:** Medium
**Related requirements:** [2.5 USB Tethering](../requirements/tasks_open.md#25-usb-tethering)

---

## Goal

Detect when an Android/iOS phone is connected via USB and sharing its mobile data connection. Auto-configure as a WAN source so the travel router can use the phone's cellular connection for internet.

---

## Background

- **Android USB tethering:** Appears as a RNDIS (usb0) or CDC-NCM network interface
- **iOS USB tethering:** Requires `usbmuxd` + `libimobiledevice` to create an `eth1` or similar interface
- **OpenWRT support:** Both require kernel modules (`kmod-usb-net-rndis`, `kmod-usb-net-cdc-ncm`) and for iOS additionally `usbmuxd`

---

## Phases

### Phase 1 — Kernel Module Detection & Auto-Install

**Required packages:**
- Android: `kmod-usb-net-rndis`, `kmod-usb-net-cdc-ncm`, `kmod-usb-net-cdc-ether`
- iOS: `usbmuxd`, `libimobiledevice`, `kmod-usb-net-ipheth`

**Approach:**
1. Add "USB Tethering" as an installable service in the Services page
2. Install the appropriate kernel modules based on user selection (Android / iOS / Both)
3. After install, the system can detect tethered devices

**Files:**
- `backend/internal/services/service_registry.go` — USB tethering service definition
- Frontend: service card with Android/iOS toggle

### Phase 2 — Device Detection

**Detection method:** Monitor `/sys/class/net/` for new USB-backed interfaces.

**Implementation:**
- Periodic check (or hotplug script) that looks for interfaces in `/sys/class/net/*/device/` that are USB devices
- Check interface naming: `usb0` (RNDIS), `eth1` (NCM/ipheth), `wwan0` (QMI — skip, that's cellular modem)
- Identify device type by examining USB device class

**API:** `GET /api/v1/network/usb-tethering/status`
```json
{
  "detected": true,
  "device_type": "android",
  "interface": "usb0",
  "is_up": true,
  "ip_address": "192.168.42.129"
}
```

### Phase 3 — Auto-Configure as WAN Source

**When:** USB tethering device detected, user confirms "Use as WAN"

**Steps:**
1. Create UCI network interface:
   ```
   uci set network.usbtether=interface
   uci set network.usbtether.proto=dhcp
   uci set network.usbtether.device=usb0
   uci set network.usbtether.metric=30
   ```
2. Add to WAN firewall zone:
   ```
   uci add_list firewall.@zone[wan].network=usbtether
   ```
3. Commit and restart network

**Metric values for priority:** WAN (eth) = 10, WWAN (wifi) = 20, USB tether = 30

### Phase 4 — Dashboard Integration

- Show USB tethering as a WAN source option in the WAN Source interplay card
- Status indicator: "USB Tether (Android)" with phone icon
- Auto-detect and notify user when phone is connected

**Files:**
- `frontend/src/pages/network/network-page.tsx` — add USB to WAN_SOURCES list
- `frontend/src/pages/dashboard/wan-source-card.tsx` — USB source type

---

## Hotplug Integration

For automatic detection, create a hotplug script:

```sh
#!/bin/sh
# /etc/hotplug.d/net/99-usb-tether
[ "$ACTION" = "add" ] || exit 0
# Check if this is a USB network device
[ -d "/sys/class/net/$INTERFACE/device" ] || exit 0
readlink -f "/sys/class/net/$INTERFACE/device" | grep -q usb || exit 0
# Notify the backend via a simple trigger file
echo "$INTERFACE" > /tmp/usb-tether-detected
```

Backend polls `/tmp/usb-tether-detected` or listens via inotify.

---

## Testing Strategy

- **Unit tests:** Mock interface detection, test UCI config generation
- **Real device test:** Connect Android phone via USB, verify interface appears
- **iOS testing:** Requires actual iOS device + usbmuxd

---

## Risks & Notes

- **iOS complexity:** iOS tethering is significantly more complex than Android; may want to support Android first
- **Hotplug reliability:** USB detection can be racy; need debouncing
- **Power:** Phone may drain battery if not charging; should warn user
- **Interface naming:** Not all USB tethering creates `usb0` — need to handle varying names
