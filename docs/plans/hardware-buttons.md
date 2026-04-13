---
title: "Plan: Hardware Buttons (GL.iNet AXT1800)"
description: "Planning / design notes: Plan: Hardware Buttons (GL.iNet AXT1800)"
updated: 2026-04-13
tags: [hardware, plan, traceability]
---

# Plan: Hardware Buttons (GL.iNet AXT1800)

**Status:** Not implemented
**Priority:** Medium
**Related requirements:** [14. Hardware Buttons](../requirements/tasks_open.md#14-hardware-buttons)

---

## Goal

Detect available hardware buttons on the router, display them in the UI, and allow users to configure what action each button triggers.

---

## Hardware Discovery (AXT1800)

The GL.iNet GL-AXT1800 has the following button-related infrastructure:

**Existing hotplug button handlers (`/etc/rc.button/`):**
- `reset` — factory reset (long press)
- `rfkill` — WiFi on/off toggle switch (the physical slide switch on the side)
- `wps` — WPS button (the small button)
- `power`, `failsafe`, `reboot` — system handlers

**The physical controls:**
- **Toggle switch** (side) → triggers `rfkill` — currently toggles all WiFi radios on/off
- **Button** (side) → triggers `wps` — currently runs scripts in `/etc/rc.wps/`
- **Reset pin-hole** → triggers `reset` — factory reset on long press

**LEDs:** `blue:run`, `white:system` (already managed by stealth mode)

---

## Phases

### Phase 1 — Button Detection API

**API:** `GET /api/v1/system/buttons`

**Detection method:** List `/etc/rc.button/` entries, cross-reference with known button types.

**Response:**
```json
{
  "buttons": [
    {
      "id": "rfkill",
      "type": "switch",
      "label": "WiFi Toggle Switch",
      "description": "Physical slide switch on the side of the device",
      "current_action": "wifi_toggle",
      "available_actions": ["wifi_toggle", "vpn_toggle", "led_toggle", "custom_script"]
    },
    {
      "id": "wps",
      "type": "button",
      "label": "Side Button",
      "description": "Push button on the side of the device",
      "current_action": "wps",
      "available_actions": ["vpn_toggle", "wifi_toggle", "led_toggle", "wps", "custom_script"]
    },
    {
      "id": "reset",
      "type": "button",
      "label": "Reset (pin-hole)",
      "description": "Recessed reset button — long press for factory reset",
      "current_action": "factory_reset",
      "available_actions": ["factory_reset"]
    }
  ]
}
```

**Implementation:**
- Read `/etc/rc.button/` directory
- Parse each script to determine current action
- For `rfkill`: check if script contains `wifi` commands → `wifi_toggle`
- For `wps`: check `/etc/rc.wps/` for custom scripts
- Store custom action config in `/etc/openwrt-travel-gui/buttons.json`

### Phase 2 — Configure Button Actions

**API:** `PUT /api/v1/system/buttons/:id/action`

**Request:** `{ "action": "vpn_toggle" }`

**Implementation:**
- Generate appropriate shell script for the chosen action
- Write to `/etc/rc.button/<id>` (for rfkill/wps)
- Or create scripts in `/etc/rc.wps/` for the WPS button

**Action scripts:**

```sh
# vpn_toggle — toggle WireGuard on/off
#!/bin/sh
[ "${ACTION}" = "released" ] || exit 0
if ip link show wg0 >/dev/null 2>&1; then
    ifdown wg0
else
    ifup wg0
fi
```

```sh
# led_toggle — toggle stealth mode
#!/bin/sh
[ "${ACTION}" = "released" ] || exit 0
for led in /sys/class/leds/*/brightness; do
    current=$(cat "$led")
    if [ "$current" -gt 0 ]; then
        echo 0 > "$led"
    else
        echo 255 > "$led"
    fi
done
```

```sh
# wifi_toggle — existing rfkill behavior (preserve as default)
```

### Phase 3 — Frontend UI

- New section in System page: "Hardware Buttons"
- Card showing each detected button with:
  - Icon (toggle switch icon, button icon, reset icon)
  - Current action label
  - Dropdown to change action
  - "Test" feedback (show last button event timestamp)
- Warning on reset button: "This button's action cannot be changed"

### Phase 4 — Long Press vs Short Press (Future)

**For WPS button only** (the toggle switch doesn't have press duration):
- Short press (< 1 second): Action A
- Long press (> 3 seconds): Action B
- Detection: The `wps` handler receives `ACTION=pressed` then `ACTION=released` — measure time between
- Store dual actions in config

---

## Testing Strategy

- **Unit tests:** Test button detection parsing, script generation
- **Real device:** Verify button events trigger actions
- **Safety:** Never overwrite `reset` button handler — always keep factory reset capability

---

## Notes

- **Backup original scripts:** Before modifying any `/etc/rc.button/` script, save backup to `/etc/openwrt-travel-gui/button-backups/`
- **Safety constraint:** Reset button action MUST NOT be user-configurable (always factory reset)
- **Other devices:** The detection should work generically; different GL.iNet models or generic OpenWRT routers will have different buttons in `/etc/rc.button/`
- **The AXT1800 toggle switch** acts as rfkill — switching it sends `TYPE=switch` with `ACTION=pressed`/`released`. This is different from a momentary button.
- **MicroSD slot** is storage, not a button — no action to configure. Could detect it for storage expansion display though.
