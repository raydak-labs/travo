---
title: "UX Overhaul Plan"
description: "Planning / design notes: UX Overhaul Plan"
updated: 2026-04-13
tags: [plan, traceability, ux]
---

# UX Overhaul Plan

> **Created:** 2026-03-25
> **Goal:** Restructure the UI so daily tasks are fast and obvious, while advanced
> features remain accessible but don't dominate the default view.
> **Principle:** A non-technical user should be able to check "is the internet up?"
> and "who is connected?" within 2 seconds of opening any page. Configuration and
> power-user features live behind tabs, accordions, or secondary views.

---

## Table of Contents

- [Network Page Restructure](#network-page-restructure)
- [Network Page Extraction](#network-page-extraction)
- [Mobile Form Fixes](#mobile-form-fixes)
- [System Page Restructure](#system-page-restructure)
- [System Page Extraction](#system-page-extraction)
- [WiFi Page Restructure](#wifi-page-restructure)
- [Dashboard Improvements](#dashboard-improvements)
- [Dark Mode Chart Fix](#dark-mode-chart-fix)
- [Confirmation Standardisation](#confirmation-standardisation)

---

## Network Page Restructure

### Problem

The network page renders 21 cards in a single vertical scroll with no grouping.
The most frequently accessed card (Connected Clients) is at position 13. A user
who wants to check "who is on my network" must scroll past WAN config, DHCP
settings, DNS config, DDNS, and DNS entries to get there.

### Solution: Tabbed Sections

Introduce a tab bar at the top of the network page with three tabs:

```
[ Status ]  [ Configuration ]  [ Advanced ]
```

**Status tab** (default, shown on page load):
1. Internet Connectivity badge (merged into WAN Source card header as a status dot)
2. WAN Source card (active interface, IP, gateway)
3. Connected Clients table (moved from position 13)
4. Interface Traffic Charts
5. Connection Uptime Log
6. Data Usage section

**Configuration tab:**
1. WAN Configuration (type, IP settings, auto-detect)
2. LAN Configuration (read-only display)
3. Network Interfaces (up/down toggles)
4. DHCP Configuration (range, lease time)
5. DNS Configuration (servers, presets)
6. DHCP Leases (active leases table)
7. DNS Entries (custom hostname mappings)
8. DHCP Reservations (static IP assignments)
9. Blocked Clients (MAC-based blocking)
10. DDNS Configuration

**Advanced tab:**
1. Firewall zone summary + port forwarding
2. IPv6 toggle and status
3. DNS over HTTPS toggle
4. Wake-on-LAN
5. Network Diagnostics (ping, traceroute, DNS lookup)
6. Speed Test
7. USB Tethering

### Implementation

**Tab component:** Use a simple controlled state with URL hash persistence so
the selected tab survives page refresh (`#status`, `#config`, `#advanced`).
Do NOT use a routing-based approach (no new routes) — keep it as local state
with hash sync.

```tsx
// network-page.tsx
const [activeTab, setActiveTab] = useState<'status' | 'config' | 'advanced'>(() => {
  const hash = window.location.hash.slice(1);
  return ['status', 'config', 'advanced'].includes(hash) ? hash : 'status';
});
```

**Tab bar styling:** Use the same pattern as the Logs page tab bar (lines 46-68
of `logs-page.tsx`) — a row of buttons with active state highlighting. This
maintains visual consistency.

**Card moves:**
- `ConnectedClients` (currently rendered at line 912-931): move to Status tab
- `DataUsageSection` (currently at line 1005): move to Status tab
- `InterfaceTrafficCharts` (currently at line 244): stays in Status tab
- `FirewallCard`, `DiagnosticsCard`, `IPv6Card`, `DoHCard`, `WoLCard`,
  `SpeedTestCard`, `USBTetheringSection`: all move to Advanced tab

**Internet Connectivity merge:** The standalone Internet Connectivity card
(lines 209-223) renders a single badge. Instead, add a coloured status dot
to the WAN Source card's `CardTitle`:

```tsx
<CardTitle className="flex items-center gap-2">
  <span className={cn("h-2 w-2 rounded-full",
    isConnected ? "bg-green-500" : "bg-red-500"
  )} />
  WAN Source
</CardTitle>
```

This saves one full card of vertical space.

### Files to modify

| File | Change |
|------|--------|
| `frontend/src/pages/network/network-page.tsx` | Add tab state, reorganise JSX into 3 tab panels |
| New: `frontend/src/pages/network/network-status-tab.tsx` | Status tab content |
| New: `frontend/src/pages/network/network-config-tab.tsx` | Configuration tab content |
| New: `frontend/src/pages/network/network-advanced-tab.tsx` | Advanced tab content |

---

## Network Page Extraction

### Problem

`network-page.tsx` is 1008 lines with ~15 inline card sections that each manage
their own state. This makes the file hard to navigate and maintain.

### Solution

Extract each self-contained section into its own component file. After
extraction, `network-page.tsx` should be under 300 lines — purely a layout
compositor that imports and arranges tab content.

**Sections to extract (by size):**

| Current location (lines) | New file | Approx lines |
|--------------------------|----------|-------------|
| 247-304 (WAN Config) | `wan-config-card.tsx` | ~60 |
| 307-353 (Network Interfaces) | `interfaces-card.tsx` | ~50 |
| 356-380 (LAN Config) | `lan-config-card.tsx` | ~30 |
| 383-456 (DHCP Config) | Already extracted: `dhcp-config-card.tsx` | — |
| 459-546 (DNS Config) | Already extracted: `dns-config-card.tsx` | — |
| 549-664 (DDNS) | Already extracted: `ddns-card.tsx` | — |
| 667-753 (DNS Entries) | `dns-entries-card.tsx` | ~90 |
| 756-865 (DHCP Reservations) | `dhcp-reservations-card.tsx` | ~110 |
| 867-910 (DHCP Leases) | `dhcp-leases-card.tsx` | ~45 |
| 912-931 (Connected Clients wrapper) | Inline in tab, uses existing `ClientsTable` | — |
| 933-981 (Uptime Log) | `uptime-log-card.tsx` | ~50 |
| 209-241 (Connectivity + WAN Source) | `wan-status-card.tsx` (merged) | ~40 |

Each extracted component owns its own hooks and local state. The parent page
file only imports and composes them.

---

## Mobile Form Fixes

### Problem

Three inline add-forms use fixed multi-column grids with no responsive
breakpoints:

1. **Firewall port forward** (`firewall-card.tsx:181`):
   `grid-cols-[1fr_auto_1fr_1fr_1fr_auto]` — 6 columns, ~45px each on mobile
2. **DHCP Reservations** (`network-page.tsx:807`):
   `grid-cols-[1fr_1fr_1fr_auto]` — 4 columns, MAC address unreadable
3. **DNS Entries** (`network-page.tsx:713`):
   `grid-cols-[1fr_1fr_auto]` — 3 columns, workable but tight

### Solution

Use responsive grid breakpoints. On mobile, stack inputs vertically:

```tsx
// Firewall example — before:
<div className="grid grid-cols-[1fr_auto_1fr_1fr_1fr_auto] gap-2 items-end">

// After:
<div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-[1fr_auto_1fr_1fr_1fr_auto] gap-2 items-end">
```

For the firewall form specifically, consider a 2-column layout on `sm` and
full 6-column on `lg`:

```
Mobile (1 col):        Tablet (2 col):          Desktop (6 col):
[Name        ]         [Name    ] [Protocol]    [Name][Proto][Ext][IP][Int][Add]
[Protocol    ]         [Ext Port] [Int IP  ]
[External Port]        [Int Port] [Add     ]
[Internal IP ]
[Internal Port]
[Add         ]
```

For DHCP reservations, consider adding a "pick from connected clients" dropdown
that auto-fills the MAC and hostname, eliminating raw MAC entry entirely.

### Files to modify

| File | Change |
|------|--------|
| `frontend/src/pages/network/firewall-card.tsx` | Responsive grid classes on add-form |
| `frontend/src/pages/network/network-page.tsx` (or extracted `dhcp-reservations-card.tsx`) | Responsive grid + client picker |
| `frontend/src/pages/network/network-page.tsx` (or extracted `dns-entries-card.tsx`) | Responsive grid |

---

## System Page Restructure

### Problem

The system page renders 13+ cards in a single scroll with no grouping. Dangerous
actions (Reboot, Shutdown, Factory Reset) sit in the middle of the page alongside
mundane settings like Timezone and NTP. The ordering does not match usage frequency.

### Solution: Grouped Sections with Visual Separators

No tabs needed — the system page has fewer cards than network. Instead, use
section headings (`<h2>`) with subtle horizontal rules to create visual groups:

```
=== At a Glance ===
  System Info + Uptime (merged)
  System Stats (CPU / Memory / Storage)

=== Configuration ===
  Time & Timezone
  NTP Configuration
  Change Password
  SSH Key Management
  LED Control
  Hardware Buttons
  Alert Thresholds

=== Maintenance ===
  Backup & Restore
  Firmware Upgrade

=== Danger Zone ===          (red-tinted separator, like GitHub's danger zone)
  Reboot
  Shutdown
  Factory Reset

=== Utilities ===
  Quick Links (LuCI, AdGuard, etc.)
```

**Merge Uptime into System Information:**
The Uptime card (lines 294-309) displays a single `<p>` tag. Move the uptime
value into the System Information card as an additional row in its info grid:

```tsx
<div className="grid grid-cols-2 gap-y-2">
  <span>Hostname</span><span>{info.hostname}</span>
  <span>Model</span><span>{info.model}</span>
  <span>Uptime</span><span>{formatUptime(stats.uptime_seconds)}</span>
  ...
</div>
```

**Danger Zone visual treatment:**
Add a red-tinted border or background to the danger section:

```tsx
<div className="rounded-lg border border-red-200 bg-red-50/50 dark:border-red-900 dark:bg-red-950/20 p-4 space-y-4">
  <h3 className="text-sm font-semibold text-red-700 dark:text-red-400">
    Danger Zone
  </h3>
  <RebootCard />
  <ShutdownCard />
  <FactoryResetCard />
</div>
```

### Files to modify

| File | Change |
|------|--------|
| `frontend/src/pages/system/system-page.tsx` | Reorder sections, add group headings, merge Uptime |

---

## System Page Extraction

### Problem

`system-page.tsx` is 1087 lines with 19 `useState` declarations and ~20 hook
imports. Large inline sections should be separate components.

### Extraction Targets

| Section | Lines | New file | State to move |
|---------|-------|----------|--------------|
| NTP Configuration | 360-456 (97 lines) | `ntp-config-card.tsx` | `ntpEnabled`, `ntpServers`, `newNtpServer` |
| Actions (Reboot/Shutdown/Factory Reset) | 525-637 (113 lines) | `danger-zone-card.tsx` | `rebootConfirm`, `shutdownConfirm` |
| LED Control | 639-770 (132 lines) | `led-control-card.tsx` | `stealthMode`, `scheduleEnabled`, `ledOnTime`, `ledOffTime` |
| Firmware Upgrade | 823-925 (103 lines) | `firmware-upgrade-card.tsx` | `firmwareFile`, `keepSettings`, `showFirmwareDialog` |
| Change Password | 927-992 (66 lines) | `change-password-card.tsx` | `currentPassword`, `newPassword`, `confirmPassword` |
| Hardware Buttons | 994-1052 (59 lines) | `hardware-buttons-card.tsx` | `buttonActions` |

After extraction, `system-page.tsx` drops from 1087 to ~500 lines and its
`useState` count drops from 19 to ~5.

---

## WiFi Page Restructure

### Problem

10 cards always render, but 5 of them (Guest, MAC Address, MAC Policy, Band
Switching, Schedule) are "set once and forget" features that clutter daily use.

### Solution: Collapsible Advanced Section

Add a disclosure section at the bottom:

```tsx
<details className="group">
  <summary className="flex cursor-pointer items-center gap-2 text-sm font-medium text-gray-600 dark:text-gray-400">
    <ChevronRight className="h-4 w-4 transition-transform group-open:rotate-90" />
    Advanced WiFi Settings
  </summary>
  <div className="mt-4 space-y-6">
    <GuestNetworkCard />
    <MACAddressCard />
    <MACPolicyCard />
    <BandSwitchingCard />
    <WiFiScheduleCard />
  </div>
</details>
```

This uses native HTML `<details>/<summary>` — no JS needed, accessible,
remembers state if the browser preserves it. For persistent state, add a
`localStorage` key: `wifi-advanced-open`.

**Mode-dependent hiding:** Add conditional rendering:

```tsx
{currentMode !== 'ap' && <SavedNetworksCard />}
{currentMode !== 'sta' && <AccessPointSection />}
```

This hides irrelevant sections when they don't apply to the current mode.

### WiFi Page Extraction

The 4 inline cards (Radio Hardware 77 lines, Current Connection 57 lines,
Saved Networks 96 lines, AP Config 152 lines) total 442 lines. Extract each:

| Section | New file |
|---------|----------|
| Radio Hardware (107-184) | `radio-hardware-card.tsx` |
| Current Connection (186-243) | `current-connection-card.tsx` |
| Saved Networks (245-341) | `saved-networks-card.tsx` |
| AP Configuration (343-495) | `ap-config-section.tsx` |

After extraction, `wifi-page.tsx` drops from 572 to ~130 lines.

---

## Dashboard Improvements

### Problem

QuickActions is below the fold (after two full-width charts). The "WiFi On"
label describes state rather than action. Inline ActionState feedback duplicates
hook-level Sonner toasts.

### Solution

**Move QuickActions above charts:**

```tsx
// dashboard-page.tsx — current order:
<Grid>...6 cards...</Grid>
<Grid>...2 charts...</Grid>
<QuickActions />                   // below fold

// New order:
<Grid>...6 cards...</Grid>
<QuickActions />                   // visible without scrolling on most screens
<Grid>...2 charts...</Grid>
```

**Fix action labels:**

```tsx
// Before:
label: wifiEnabled ? 'WiFi On' : 'WiFi Off'

// After:
label: wifiEnabled ? 'Disable WiFi' : 'Enable WiFi'
```

**Remove duplicate feedback:** Delete the local `ActionState` icon logic from
QuickActions. The hook mutations already fire Sonner toasts on success/error.
The button only needs a loading spinner while `isPending` is true:

```tsx
<Button disabled={mutation.isPending}>
  {mutation.isPending ? <Loader2 className="animate-spin" /> : <Icon />}
  {label}
</Button>
```

**Replace `window.confirm` with Dialog:** Add a confirmation Dialog for Reboot
in QuickActions, matching the pattern already used in `header.tsx`.

### Files to modify

| File | Change |
|------|--------|
| `frontend/src/pages/dashboard/dashboard-page.tsx` | Move QuickActions above charts |
| `frontend/src/pages/dashboard/quick-actions.tsx` | Fix labels, remove ActionState, add Dialog for reboot |

---

## Dark Mode Chart Fix

### Problem

`bandwidth-chart.tsx` and `network-chart.tsx` use hardcoded hex colours in
Recharts props. The tooltip background (`rgba(0,0,0,0.8)`) is always black, and
grid lines (`#9ca3af`) may be invisible on dark backgrounds.

### Solution

Use CSS custom properties that respond to the Tailwind dark mode class:

```css
/* In the global CSS (index.css or tailwind layer) */
:root {
  --chart-grid: #e5e7eb;          /* gray-200 */
  --chart-tooltip-bg: white;
  --chart-tooltip-text: #111827;  /* gray-900 */
}
.dark {
  --chart-grid: #374151;          /* gray-700 */
  --chart-tooltip-bg: #1f2937;   /* gray-800 */
  --chart-tooltip-text: #f9fafb; /* gray-50 */
}
```

Then reference them in chart configs:

```tsx
<CartesianGrid stroke="var(--chart-grid)" />
<Tooltip contentStyle={{
  backgroundColor: 'var(--chart-tooltip-bg)',
  color: 'var(--chart-tooltip-text)',
  border: '1px solid var(--chart-grid)',
}} />
```

### Files to modify

| File | Change |
|------|--------|
| `frontend/src/index.css` | Add CSS custom properties for chart colours |
| `frontend/src/pages/dashboard/bandwidth-chart.tsx` | Replace hardcoded hex with CSS vars |
| `frontend/src/pages/dashboard/network-chart.tsx` | Replace hardcoded hex with CSS vars |

---

## Confirmation Standardisation

### Problem

Five destructive actions use three different confirmation patterns:

| Action | Current pattern | Quality |
|--------|----------------|---------|
| Factory Reset | Full `<Dialog>` with warning text | Best |
| Firmware Upgrade | Full `<Dialog>` with warning text | Good |
| Reboot (system page) | Inline badge + 2 small buttons | Weak |
| Shutdown (system page) | Inline badge + 2 small buttons | Weak |
| Restore from Backup | `window.confirm()` (native browser) | Worst |
| Reboot (QuickActions) | `window.confirm()` | Worst |

### Solution

All 6 destructive actions must use the same pattern:

```tsx
<Dialog open={showConfirm} onOpenChange={setShowConfirm}>
  <DialogContent>
    <DialogHeader>
      <DialogTitle className="flex items-center gap-2">
        <AlertTriangle className="h-5 w-5 text-red-500" />
        {title}
      </DialogTitle>
      <DialogDescription>{description}</DialogDescription>
    </DialogHeader>
    {warningText && (
      <div className="rounded bg-red-50 p-3 text-sm text-red-700
                      dark:bg-red-950 dark:text-red-300">
        {warningText}
      </div>
    )}
    <DialogFooter>
      <Button variant="ghost" onClick={() => setShowConfirm(false)}>Cancel</Button>
      <Button variant="destructive" onClick={onConfirm} disabled={isPending}>
        {isPending ? 'Processing...' : confirmLabel}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
```

Consider extracting this as a reusable `<ConfirmDialog>` component that accepts
`title`, `description`, `warningText`, `confirmLabel`, `onConfirm`, and
`variant` props, since it appears 6 times.

### Files to modify

| File | Change |
|------|--------|
| New: `frontend/src/components/ui/confirm-dialog.tsx` | Reusable confirmation dialog |
| `frontend/src/pages/system/system-page.tsx` | Reboot/Shutdown/Restore use ConfirmDialog |
| `frontend/src/pages/dashboard/quick-actions.tsx` | Reboot uses ConfirmDialog |

---

## Execution Order

Recommended order to minimise merge conflicts and allow incremental review:

1. **Confirmation Standardisation** — small, self-contained, fixes real bugs
2. **Dashboard Improvements** — small, high-visibility, improves first impression
3. **Dark Mode Chart Fix** — small CSS-only change
4. **System Page Extraction** — extract components without restructuring layout
5. **System Page Restructure** — reorder and group using extracted components
6. **Network Page Extraction** — extract before restructuring
7. **Network Page Restructure** — add tabs using extracted components
8. **Mobile Form Fixes** — can be done alongside or after extraction
9. **WiFi Page Restructure** — collapsible section + extraction
