---
title: "Frontend traveler IA implementation"
description: "Implement sidebar-only nav, label renames, storage defaults, tab removal, clients preview, page-section collapse"
updated: 2026-07-24
tags: [plan, frontend, ux, ia, implementation]
---

# Frontend traveler IA Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the traveler IA from [`2026-07-24-frontend-traveler-ia.md`](2026-07-24-frontend-traveler-ia.md): sidebar-only WiFi/Network navigation, collapsed-by-default groups, leaf titles, clients preview, and in-place page-section collapse — without deleting features or moving cards between routes.

**Architecture:** Keep existing TanStack routes. Remove in-page tab bars that mirror sidebar children. Drive group defaults via `nav-config` + bumped localStorage key; `useSidebarGroups` already auto-expands active groups. Add a small `PageSection` collapsible wrapper for dense panels. Update shell titles to leaf labels.

**Tech Stack:** React + TypeScript + Vite + Vitest + Testing Library, existing `Collapsible` primitive, TanStack Router.

## Global Constraints

- Spec: [`2026-07-24-frontend-traveler-ia.md`](2026-07-24-frontend-traveler-ia.md); nav norms: [`../ui-theming.md`](../ui-theming.md) § Navigation patterns.
- No feature deletion; no moving cards between Status/Setup/Advanced routes.
- Storage key bump to `otg-sidebar-groups-v2`; `defaultOpen` all `false`.
- Labels: Wireless → Connect, Configuration → Setup, Installed services → Apps.
- Clients preview on Network Status: max **5** rows + View all → `/clients`.
- Logs System/Kernel tabs stay.
- TDD: failing test → minimal code → pass. Commit after each task.
- Finish: from repo root `make lint`, `make test`, `make build`.
- Follow `docs/ui-theming.md` color rules (no light-only text colors).

---

## File map

| File | Role |
|------|------|
| `frontend/src/components/layout/nav-config.ts` | Labels, `defaultOpen`, `STORAGE_KEY_GROUPS` → v2 |
| `frontend/src/components/layout/use-sidebar-groups.ts` | Already auto-expands; keep; tests cover |
| `frontend/src/components/layout/sidebar.tsx` | Fix `groupOpen[id] ?? true` → `?? false` |
| `frontend/src/router.tsx` | Leaf shell titles for WiFi/Network children |
| `frontend/src/pages/wifi/wifi-page.tsx` | Drop tab bar; render active panel only |
| `frontend/src/pages/network/network-page.tsx` | Same |
| `frontend/src/pages/wifi/wifi-page-tab-bar.tsx` | Delete after unused |
| `frontend/src/pages/network/network-page-tab-bar.tsx` | Delete after unused |
| `frontend/src/pages/wifi/wifi-*-panel.tsx` | Drop tabpanel a11y tied to removed tabs |
| `frontend/src/pages/network/network-page-*-panel.tsx` | Same + clients preview + Setup collapse |
| `frontend/src/components/ui/page-section.tsx` | New collapsible section wrapper |
| `frontend/src/pages/vpn/*`, `system/*` | Collapse secondary blocks |
| Tests under `__tests__/` next to changed modules | |

---

### Task 1: Nav labels, defaults, storage v2

**Files:**
- Modify: `frontend/src/components/layout/nav-config.ts`
- Modify: `frontend/src/components/layout/sidebar.tsx` (`?? false`)
- Modify: `frontend/src/components/layout/__tests__/sidebar.test.tsx`
- Create: `frontend/src/components/layout/__tests__/nav-config.test.ts`

**Interfaces:**
- Produces: `STORAGE_KEY_GROUPS = 'otg-sidebar-groups-v2'`; labels Connect / Setup / Apps; `defaultOpen` all false; `loadSidebarGroupState()` returns all false when empty.

- [ ] **Step 1: Write failing tests**

```ts
// nav-config.test.ts
import { describe, it, expect, beforeEach } from 'vitest';
import {
  NAV_ENTRIES,
  loadSidebarGroupState,
  saveSidebarGroupState,
} from '@/components/layout/nav-config';

describe('nav-config traveler IA', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('uses Connect / Setup / Apps labels', () => {
    const wifi = NAV_ENTRIES.find((e) => e.id === 'wifi');
    const network = NAV_ENTRIES.find((e) => e.id === 'network');
    const services = NAV_ENTRIES.find((e) => e.id === 'services');
    expect(wifi?.kind === 'group' && wifi.items.map((i) => i.label)).toEqual([
      'Connect',
      'Advanced',
    ]);
    expect(network?.kind === 'group' && network.items.map((i) => i.label)).toEqual([
      'Status',
      'Setup',
      'Advanced',
    ]);
    expect(services?.kind === 'group' && services.items[0]?.label).toBe('Apps');
  });

  it('defaults all groups collapsed when storage empty', () => {
    expect(loadSidebarGroupState()).toEqual({
      wifi: false,
      network: false,
      services: false,
      system: false,
    });
  });

  it('ignores legacy otg-sidebar-groups key', () => {
    localStorage.setItem(
      'otg-sidebar-groups',
      JSON.stringify({ wifi: true, network: true, services: true, system: true }),
    );
    expect(loadSidebarGroupState().wifi).toBe(false);
  });

  it('persists under v2 key', () => {
    saveSidebarGroupState('wifi', true);
    expect(localStorage.getItem('otg-sidebar-groups-v2')).toContain('"wifi":true');
  });
});
```

Update `sidebar.test.tsx` expectations: `Wireless` → `Connect`; assert on `/dashboard` with empty storage that WiFi child links are not visible until group opened (or group trigger `aria-expanded=false` if children stay in DOM hidden — match Collapsible behavior: content not visible when closed).

- [ ] **Step 2: Run tests — expect FAIL**

```bash
cd frontend && pnpm exec vitest run src/components/layout/__tests__/nav-config.test.ts src/components/layout/__tests__/sidebar.test.tsx
```

- [ ] **Step 3: Implement**

In `nav-config.ts`:
- Rename labels as above.
- `STORAGE_KEY_GROUPS = 'otg-sidebar-groups-v2'`.
- `defaultOpen`: all `false`.

In `sidebar.tsx`:
- `open={groupOpen[entry.id] ?? false}` (was `?? true`).

- [ ] **Step 4: Run tests — expect PASS**

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/layout/nav-config.ts frontend/src/components/layout/sidebar.tsx frontend/src/components/layout/__tests__/
git commit -m "$(cat <<'EOF'
feat(ui): traveler sidebar labels and collapsed defaults

EOF
)"
```

---

### Task 2: Leaf shell titles for WiFi / Network

**Files:**
- Modify: `frontend/src/router.tsx`
- Create or extend: `frontend/src/router/__tests__/shell-titles.test.ts` (or update existing router test if any)

**Interfaces:**
- Produces: `/wifi` → title `Connect`; `/wifi/advanced` → `Advanced`; `/network` → `Status`; `/network/configuration` → `Setup`; `/network/advanced` → `Advanced`. Services children keep `Services / X`.

- [ ] **Step 1: Write failing test** that mounts routes (or asserts `shellPage` wiring via exported title map if you extract one). Prefer small helper:

```ts
// frontend/src/components/layout/shell-titles.ts
export function shellTitleForPath(pathname: string): string {
  const map: Record<string, string> = {
    '/dashboard': 'Dashboard',
    '/wifi': 'Connect',
    '/wifi/advanced': 'Advanced',
    '/network': 'Status',
    '/network/configuration': 'Setup',
    '/network/advanced': 'Advanced',
    '/clients': 'Clients',
    '/vpn': 'VPN',
    '/services': 'Services',
    '/services/tailscale': 'Services / Tailscale',
    '/services/speedtest': 'Services / Speedtest',
    '/services/sqm': 'Services / SQM',
    '/system': 'System',
    '/logs': 'Logs',
  };
  return map[pathname] ?? 'Travo';
}
```

Test `shellTitleForPath` exhaustively for the map. Wire `router.tsx` to use these strings in `shellPage(...)`.

- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Add helper + update `router.tsx` comments/titles**
- [ ] **Step 4: Run — PASS**
- [ ] **Step 5: Commit** `feat(ui): leaf titles for WiFi and Network routes`

---

### Task 3: Remove WiFi in-page tab bar

**Files:**
- Modify: `frontend/src/pages/wifi/wifi-page.tsx`
- Modify: `frontend/src/pages/wifi/wifi-wireless-panel.tsx`, `wifi-advanced-panel.tsx` (drop `role="tabpanel"` / unused tabId props if unused)
- Delete: `frontend/src/pages/wifi/wifi-page-tab-bar.tsx`
- Modify: `frontend/src/pages/wifi/__tests__/wifi-page.test.tsx`

**Interfaces:**
- Consumes: pathname → show only Connect or Advanced panel (no `hidden` twin in DOM preferred: conditional render).
- Produces: no `role="tablist"` on WiFi page.

- [ ] **Step 1: Failing test** — assert no tab role for Wireless/Advanced; both routes still show correct panel content (reuse existing content assertions).

```ts
it('has no in-page tablist mirroring sidebar', () => {
  renderWifiPage('/wifi');
  expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
  expect(screen.queryByRole('tab', { name: /Wireless|Connect|Advanced/i })).not.toBeInTheDocument();
});
```

- [ ] **Step 2: Run — FAIL** (tabs still present)
- [ ] **Step 3: Implement** — remove `WifiPageTabBar`; `className="space-y-6"` on page root; render one panel by pathname; simplify panel props (id optional).
- [ ] **Step 4: Run wifi page tests — PASS**
- [ ] **Step 5: Commit** `feat(ui): remove WiFi page tab bar`

---

### Task 4: Remove Network in-page tab bar

**Files:**
- Modify: `frontend/src/pages/network/network-page.tsx` + three panels
- Delete: `frontend/src/pages/network/network-page-tab-bar.tsx`
- Modify: `frontend/src/pages/network/__tests__/network-page.test.tsx`
- Optionally keep `network-path-utils.ts` for pathname → section if useful for conditional render

- [ ] **Step 1: Failing test** — no tablist; Status/Setup/Advanced routes still mount correct panels
- [ ] **Step 2: Run — FAIL**
- [ ] **Step 3: Implement** — same pattern as WiFi; root `space-y-6`
- [ ] **Step 4: PASS**
- [ ] **Step 5: Commit** `feat(ui): remove Network page tab bar`

Also update deferred note in [`2026-07-20-ui-consistency.md`](2026-07-20-ui-consistency.md): Tabbed page row → “former tabbed pages now `space-y-6` after traveler IA”.

---

### Task 5: Network Status clients preview

**Files:**
- Modify: `frontend/src/pages/network/network-page-status-panel.tsx`
- Modify: `frontend/src/pages/network/clients-table.tsx` (optional `limit` prop) **or** slice in panel
- Modify: `frontend/src/pages/network/__tests__/network-page.test.tsx`

**Interfaces:**
- Produces: at most 5 client rows; link `View all` → `/clients` (TanStack `Link`).

- [ ] **Step 1: Failing test** with mocked >5 clients — expect 5 rows + link to `/clients`
- [ ] **Step 2: FAIL**
- [ ] **Step 3: Implement** preview + link in card header/footer
- [ ] **Step 4: PASS**
- [ ] **Step 5: Commit** `feat(ui): Network Status clients preview links to Clients`

---

### Task 6: `PageSection` primitive + WiFi Advanced collapse

**Files:**
- Create: `frontend/src/components/ui/page-section.tsx`
- Create: `frontend/src/components/ui/__tests__/page-section.test.tsx`
- Modify: `frontend/src/pages/wifi/wifi-advanced-panel.tsx`

**Interfaces:**

```tsx
type PageSectionProps = {
  title: string;
  defaultOpen?: boolean; // default false
  children: React.ReactNode;
};
```

Use existing `Collapsible` / `CollapsibleTrigger` / `CollapsibleContent`. Trigger shows title + chevron; content wraps children. Default `defaultOpen={false}`.

- [ ] **Step 1: Test** — default closed (content not visible); open on click
- [ ] **Step 2: FAIL**
- [ ] **Step 3: Implement primitive**
- [ ] **Step 4: Wrap each WiFi Advanced card** in `PageSection` with titles matching card purpose (DNS tools, Radio hardware, …). Optional short intro outside sections.
- [ ] **Step 5: PASS + commit** `feat(ui): PageSection collapse for WiFi Advanced`

---

### Task 7: Network Setup + Advanced + VPN/System disclosure

**Files:**
- Modify: `network-page-configuration-panel.tsx` — WAN, LAN, Interfaces **open**; wrap DHCP/DNS/entries/reservations/leases in `PageSection` default closed
- Modify: `network-page-advanced-panel.tsx` — Failover open; wrap remaining cards in `PageSection` default closed
- Modify: VPN page secondary cards (split tunnel, leak, verify, speed) → `PageSection`
- Modify: System page maintenance/firmware/SSH/danger → `PageSection` (glance + password/timezone stay open)
- Tests: assert collapsed sections’ titles present; heavy content not visible until expand (sample 1–2 per page)

- [ ] **Step 1–4:** TDD per page cluster; keep commits split if large (`feat(ui): collapse Network Setup sections`, etc.)
- [ ] **Final commit(s)** for this task

---

### Task 8: Auto-expand + Dashboard CTA regression tests

**Files:**
- Extend: `frontend/src/components/layout/__tests__/sidebar.test.tsx`
- Dashboard CTA tests if links exist (`dashboard-page.test.tsx`)

- [ ] **Step 1: Tests**
  - Empty storage + path `/wifi/advanced` → WiFi group expanded (`aria-expanded=true` on WiFi trigger) and Advanced link current
  - Path `/dashboard` → all group triggers `aria-expanded=false`
  - Dashboard link to WiFi (if any) navigates and expands WiFi group

- [ ] **Step 2–4:** Fix any gaps in `useSidebarGroups` only if tests fail (behavior mostly exists)
- [ ] **Step 5: Commit** `test(ui): sidebar auto-expand and first-visit collapse`

---

### Task 9: Verify finish

- [ ] **Step 1:** Manual mobile-width check (DevTools): Dashboard → Open menu → WiFi Connect ≤ 3 taps; VPN toggle via quick action or nav ≤ 3 taps
- [ ] **Step 2:**

```bash
make lint && make test && make build
```

Expected: all pass.

- [ ] **Step 3: Commit** any leftover doc tweaks only if needed; otherwise done.

---

## Spec coverage checklist

| Spec item | Task |
|-----------|------|
| Labels Connect/Setup/Apps | 1 |
| defaultOpen collapsed + v2 key | 1 |
| Auto-expand active group | 8 (exists; tested) |
| Leaf titles | 2 |
| Kill WiFi/Network tab bars | 3–4 |
| Clients preview ≤5 + link | 5 |
| Page section collapse | 6–7 |
| No card route moves | all |
| Logs tabs untouched | 3–4 (do not touch Logs) |
| Success criteria / tap budget | 8–9 |

## Placeholder scan

None intentional. If `shell-titles.ts` extraction feels heavy, inline titles in `router.tsx` and test via a tiny exported `WIFI_NETWORK_SHELL_TITLES` constant instead — still no TBD.
