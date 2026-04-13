# Frontend refactor notes (sidebar, shell, and page splits)

## What changed

### Dependencies

- **`react-hook-form`**, **`@hookform/resolvers`**, **`zod`**, **`@radix-ui/react-collapsible`**, and related packages are listed in `frontend/package.json`. After pulling, run **`pnpm install`** at the repo root so Vitest and Vite resolve these modules.

### Vite production bundle (`frontend/vite.config.ts`)

- **`build.rollupOptions.output.manualChunks`** groups React (+ `scheduler`), TanStack, Recharts, Lucide, and RHF/Zod into named vendor chunks. Remaining `node_modules` are not forced into a catch-all bucket so Rollup does not emit circular chunk graphs (which occurred with a single `vendor-misc` or isolated Radix chunk).

### Sidebar navigation (`components/layout/`)

- **`nav-config.ts`** — `NAV_ENTRIES`: **WiFi** and **Network** are separate collapsible groups (each with sub-routes); **Clients** is a leaf. `isRouteActive()` uses **exact** path matching for every group child (so `/wifi` does not light up on `/wifi/advanced`), keeps `/services` vs `/services/tailscale` behavior, and `flattenNavRoutes()` for the collapsed icon rail; **localStorage** `otg-sidebar-groups`.
- **`sidebar.tsx`** — Renders expanded navigation from `NAV_ENTRIES`, or a flat icon rail when the desktop sidebar is collapsed (not in the mobile drawer).
- **`sidebar-nav-group.tsx`** — Collapsible category using shadcn `Collapsible` (Radix); sub-links are indented with a left border.
- **`use-sidebar-collapsed.ts`** — Persists desktop **sidebar collapsed** state (`otg-sidebar-collapsed`).
- **`use-sidebar-groups.ts`** — Hook for collapsible group open state, localStorage sync, and auto-expand when the active route is inside a group.
- **`app-shell.tsx`** — Uses `useSidebarCollapsed()` so collapse state survives reloads; mobile Sheet + hamburger flow unchanged.
- **`theme-context.ts`** — `ThemeContext` plus `Theme` / `ThemeContextValue` types.
- **`use-theme.ts`** — `useTheme()` hook (import from here or `@/components/layout/use-theme` in `App` / `Header`).
- **`theme-provider.tsx`** — `ThemeProvider` only; satisfies `react-refresh/only-export-components` when paired with the files above.
- **`components/ui/collapsible.tsx`** — Radix Collapsible primitives aligned with other shadcn-style wrappers.
- **`index.ts`** — Barrel exports for the layout feature.
- **`router.tsx`** — Route tree + `createRouter` only; **`LazyPageBoundary`** (`lazy-page-boundary.tsx`) wraps lazy pages in ErrorBoundary + Suspense + **Skeleton** fallback; **`router/lazy-loaded-pages.tsx`** holds `lazy()` imports; **`router/route-guards.ts`** exports `requireAuth` / `requireSetupComplete`; `shellPage()` DRYs `AppShell` + boundary + page for each protected route.

### UI primitives (`components/ui/`)

- **`badge.tsx`**, **`button.tsx`** — Export the component and `export type` for props only; CVA variant helpers are not re-exported (unused externally; keeps react-refresh clean).

### System page (`pages/system/`)

The former monolithic **`system-page.tsx`** is split into focused sections (each file is a single responsibility, most under ~150 lines):

- **`system-at-a-glance-section.tsx`** — System information + CPU/memory/storage stats (hooks: `useSystemInfo`, `useSystemStats`).
- **`system-timezone-card.tsx`** — Timezone display and edit flow (`useTimezone`, `useSetTimezone`).
- **`system-backup-restore-card.tsx`** — Backup download, restore file picker, and restore **ConfirmDialog**.
- **`system-power-section.tsx`** — Danger zone actions plus reboot / shutdown / factory **ConfirmDialog**s (owns dialog state so buttons and dialogs stay wired correctly).
- **`system-quick-links-card.tsx`** — LuCI / AdGuard links; opens **`adguard-config-editor-dialog.tsx`** for YAML edit + save.
- **`adguard-config-editor-dialog.tsx`** — AdGuardHome.yaml textarea + Cancel / Save & Restart.
- **`ssh-keys-list.tsx`**, **`ssh-key-add-form.tsx`** — Key rows + delete; RHF add-key textarea + submit.
- **`ssh-keys-card.tsx`** — Hooks + composes list + add form.
- **`system-page.tsx`** — Composes the above plus existing cards (`NTPConfigCard`, `ChangePasswordCard`, etc.).
- **`led-stealth-status-panel.tsx`** — Stealth toggle + per-LED on/off list.
- **`led-schedule-form.tsx`** — LED on/off time schedule (RHF).
- **`led-control-card.tsx`** — LED hooks + composes stealth panel + schedule form.
- **`hardware-button-actions.ts`** — Action labels + select options for physical buttons.
- **`hardware-buttons-summary-view.tsx`** — Read-only button → action rows + Edit.
- **`hardware-buttons-edit-form.tsx`** — Field-array edit + save / cancel.
- **`hardware-buttons-card.tsx`** — Hooks + view/edit toggle.
- **`firmware-upgrade-confirm-dialog.tsx`** — Destructive flash confirmation copy + actions.
- **`firmware-upgrade-form-fields.tsx`** — File picker, keep-settings toggle, flash trigger (`FirmwareUpgradeCard`).
- **`firmware-upgrade-card.tsx`** — RHF + mutation + composes fields + confirm dialog.
- **`change-password-card-summary.tsx`**, **`change-password-edit-fields.tsx`** — `ChangePasswordCard` read-only vs edit form fields.
- **`alert-threshold-slider.tsx`** — Single range input row (`AlertThresholdsCard`).

### Clients (`components/clients/` + `pages/clients/`)

- **`reserve-ip-form.tsx`** — Static DHCP reserve inline form (RHF + `dhcpReservationFormSchema`).
- **`client-row.tsx`** — One table row (alias, actions, traffic).
- **`clients-dhcp-reservations-card.tsx`** — Static reservations table.
- **`index.ts`** — Barrel exports for the clients feature.
- **`clients-filter.ts`** — `filterClientsBySearch()` for connected-clients table.
- **`clients-search-bar.tsx`**, **`clients-connected-table.tsx`** — Search row + loading / empty / table (`ClientsConnectedClientsCard`).
- **`clients-connected-clients-card.tsx`** — Hooks, reserve flow, composes search + table + `ReserveIpForm`.
- **`clients-page.tsx`** — Composes `ClientsConnectedClientsCard` + `ClientsDhcpReservationsCard`.

### Dashboard (`pages/dashboard/`)

- **`network-chart-utils.ts`** — `computeNetworkRates()` for throughput Recharts data (`NetworkChart`).
- **`quick-actions-button-row.tsx`** — WiFi enable/disable, restart WiFi, VPN toggle, reboot trigger (`QuickActions`).
- **`quick-actions.tsx`** — Data hooks, reboot `ConfirmDialog`, composes the button row.

### WireGuard VPN (`pages/vpn/`)

- **`wireguard-import-profile-file.ts`** — `applyWireguardImportFile()` reads `.conf` into import RHF (`WireguardSection`).
- **`wireguard-install-prompt.tsx`** — Not-installed callout + link to Services (used by `WireguardSection`).
- **`wireguard-utils.ts`** — `formatWireguardHandshakeTime()` for live `wg` peer display.
- **`wireguard-import-profile-form.tsx`** — Import profile form (file + textarea + submit).
- **`wireguard-status-toggle-row.tsx`** — Status badge + enable `Switch` + applying spinner.
- **`wireguard-status-detail-text.tsx`** — Human-readable status detail when not toggling.
- **`wireguard-connection-stats-panels.tsx`** — Live `wg` peer grid vs fallback RX/TX from `VpnStatus`.
- **`wireguard-config-peers-list.tsx`** — UCI peers list from `WireguardConfig`.
- **`wireguard-status-and-config-peers.tsx`** — Composes the four pieces above.
- **`wireguard-profiles-kill-import.tsx`** — Saved profiles list, kill switch, import form.
- **`wireguard-card-body-types.ts`** — Shared props type for the composed body.
- **`wireguard-card-body.tsx`** — Composes status + profiles/kill/import.
- **`wireguard-section.tsx`** — Data hooks, `useForm` for import, `OperationProgressDialog`, and not-installed / loading states only (~140 lines).

### Wi‑Fi (`pages/wifi/`)

- **`wifi-radio-hardware-card.tsx`** — Radio list, band labels, role `Select`.
- **`wifi-current-connection-card.tsx`** — STA connection status, disconnect, scan / hidden-network dialogs (hooks colocated).
- **`wifi-saved-networks-card.tsx`** — Auto-reconnect, priority reorder, delete saved networks.
- **`wifi-page-tab-bar.tsx`**, **`wifi-page-types.ts`** — In-page tabs **Wireless** / **Advanced**, synced to `/wifi` and `/wifi/advanced`.
- **`wifi-wireless-panel.tsx`** — Captive portal, mode, radios, STA connection/saved networks, AP config (mode gating).
- **`wifi-advanced-panel.tsx`** — Guest WiFi, MAC, policy, band, schedule cards.
- **`wifi-page.tsx`** — Tab state from router + composes tab bar and panels.
- **`wifi-mode-options.ts`** — `WIFI_MODE_OPTIONS`, `getWifiModeLabel()` (`WifiModeCard`).
- **`wifi-mode-switch-dialog.tsx`** — Confirm switch away from repeater path (`WifiModeCard`).
- **`wifi-mode-card.tsx`** — Mode tiles, repeater wizard trigger, composes switch dialog.
- **`ap-config-normalize.ts`**, **`ap-radio-form-header-row.tsx`**, **`ap-radio-form-credentials-and-actions.tsx`**, **`ap-radio-form-fields.tsx`** (composes header + credentials / actions), **`ap-radio-disable-dialog.tsx`** (last-AP warning), **`ap-radio-section.tsx`** (RHF + mutations + QR dialog wiring), **`ap-config-card.tsx`** — Per-radio AP form and card shell.
- **`lib/wifi-band.ts`** — `formatWifiBandLabel`, `normalizeWifiBandKey` for scan + connect UIs (UCI `2g`/`5g` still use local labels in AP components).
- **`wifi-connect-utils.ts`** — `toWifiBand`, signal quality, `buildBandOptionsFromGroup`, default band pick.
- **`wifi-connect-dialog-form.tsx`** — Scan connect header + band radios + password + actions.
- **`wifi-connect-dialog.tsx`** — RHF + overlay / embedded shell, composes form.
- **`wifi-scan-list-utils.ts`** — AP tooltip, `groupScanNetworks`.
- **`wifi-scan-list-per-band-signals.tsx`** — Per-band best signal row for a grouped network.
- **`wifi-scan-list.tsx`** — Scan toolbar, loading / empty / list UI (`WifiScanDialog`, repeater wizard).
- **`wifi-hidden-network-constants.ts`** — Encryption presets + default form values.
- **`wifi-hidden-network-dialog-fields.tsx`** — SSID, encryption, optional password, inline errors.
- **`wifi-hidden-network-dialog-form.tsx`** — `form` shell + fields + Cancel / Connect.
- **`wifi-hidden-network-dialog.tsx`** — Trigger button, dialog shell, `useWifiConnect` + RHF.
- **`guest-wifi-encryption.ts`** — Allowed guest encryption values + normalize from API.
- **`guest-wifi-enabled-fields.tsx`** — SSID, encryption, password when guest WiFi is on.
- **`guest-network-card.tsx`** — Guest WiFi hooks, enable switch, save.
- **`mac-policy-table.tsx`** — Per-network MAC policy rows + delete.
- **`mac-policy-add-form.tsx`** — Add policy RHF row; parent passes mutate `onSuccess` via callback for reset.
- **`mac-policy-card.tsx`** — Hooks, loading skeleton, composes table + form.
- **`mac-address-utils.ts`** — `generateRandomMac()` for clone UI.
- **`mac-address-clone-block.tsx`** — STA summary + custom MAC field + action row (`MACAddressCard` maps interfaces).
- **`wifi-schedule-form-fields.tsx`** — Schedule toggle + on/off time inputs (`WiFiScheduleCard`).

### App header (`components/layout/`)

- **`header.tsx`** — Title row, theme toggle, composes toolbar pieces (~45 lines).
- **`header-router-status.tsx`** — Hostname label + green/red connectivity dot (`SystemInfo` + error state).
- **`header-notifications-menu.tsx`** — Bell, dropdown list, unread badge (`useAlerts`), click-outside close.
- **`header-overflow-menu.tsx`** — ⋮ menu: reboot / shutdown / logout + confirmation **Dialog**s (`useReboot`, `useShutdown`).
- **`header-alert-severity.ts`**, **`header-format-alert-time.ts`** — Small helpers for notification rows.

### Repeater wizard (`components/wifi/repeater-wizard/`)

Import path unchanged: `@/components/wifi/repeater-wizard` → **`index.tsx`**.

- **`types.ts`**, **`map-encryption.ts`** — Shared types and scan→UCI encryption mapping.
- **`use-repeater-wizard.ts`** — State, scan/connect/mode/AP mutations, `handleApply`, derived flags.
- **`step-indicator.tsx`**, **`select-upstream-step.tsx`**, **`configure-ap-step.tsx`**, **`review-step.tsx`**, **`done-step.tsx`** — One concern per step UI.

### Tests

- **`__tests__/sidebar.test.tsx`** — Expectations for grouped labels, sub-routes, and route fixtures.

### Firewall (`pages/network/`)

- **`firewall-policy-badge.tsx`** — Zone policy → `Badge` variant.
- **`firewall-zones-section.tsx`** — Read-only zones table + loading / empty.
- **`firewall-port-forward-rules-table.tsx`** — Existing DNAT rules + delete.
- **`firewall-port-forward-add-form.tsx`** — RHF + Zod add row (`portForwardFormSchema`); **`firewall-port-forward-add-form-grid.tsx`** — field grid + submit.
- **`firewall-port-forward-section.tsx`** — Composes table + form + skeleton.
- **`firewall-card.tsx`** — Hooks + `Card` shell only.

### Tailscale (`pages/services/`)

- **`tailscale-peer-row.tsx`** — Single peer row + “Use as exit”.
- **`tailscale-auth-section.tsx`** — Pre-auth key form + auth URL link.
- **`tailscale-logged-in-panel.tsx`** — Device summary, exit node clear, SSH toggle, peers list + WireGuard notice.
- **`tailscale-section.tsx`** — Install gate, loading, progress dialogs, enable switch, composes the above.

### Services list (`pages/services/`)

- **`service-card.constants.ts`** — Icons, state → `Badge` variant, state labels.
- **`service-card-action-buttons.tsx`** — Install / start / stop / remove / Tailscale / AdGuard dashboard links.
- **`service-card.tsx`** — Card layout, auto-start, AdGuard DNS toggle when running.
- **`services-installed-card.tsx`** — Installed-services grid / skeleton / empty (`ServicesPage`).
- **`wireguard-post-install-dialog.tsx`** — Post-install VPN setup prompt (`ServicesPage`).
- **`use-install-log-stream.ts`** — SSE line buffer, status, scroll ref, close handler (`InstallLogDialog`).
- **`install-log-dialog.tsx`** — Dialog shell + log `<pre>` + footer (`useInstallLogStream`).

### Logs (`pages/logs/`)

- **`logs-constants.ts`** — Service/level filter presets, level colors, default form values, `LogTab` type.
- **`logs-level-badge.tsx`** — Colored level chip in the log stream.
- **`logs-toolbar-tabs-and-search.tsx`** — System / Kernel tabs, line filter, download + refresh.
- **`logs-toolbar-service-filters.tsx`**, **`logs-toolbar-level-filters.tsx`** — Service chips + level chips (system log only).
- **`logs-toolbar.tsx`** — Composes the three toolbars above.
- **`logs-text-view.tsx`** — Skeleton / `<pre>` stream + line count footer.
- **`logs-page.tsx`** — `useForm` + `useSystemLogs` / `useKernelLogs`, filtering, scroll-to-bottom.

### Network: DHCP/DNS, data usage (`pages/network/`)

- **`network-page-types.ts`** — `NetworkSectionTab` union.
- **`network-page-tab-bar.tsx`** — Status / Configuration / Advanced tab strip (`aria-controls` wired to panels).
- **`network-page-status-panel.tsx`**, **`network-page-configuration-panel.tsx`**, **`network-page-advanced-panel.tsx`** — Tab panel content.
- **`network-path-utils.ts`** — Maps `/network`, `/network/configuration`, `/network/advanced` ↔ tab ids.
- **`network-page.tsx`** — Tab changes call `navigate()`; `useNetworkStatus` / `useBlockedClients`; composes panels.
- **`dhcp-pool-settings-card.tsx`** — DHCP pool RHF form (`useDHCPConfig`, `useSetDHCPConfig`).
- **`dhcp-pool-form-fields.tsx`** — Start/limit inputs + lease `Select` (composed by `DhcpPoolSettingsCard`).
- **`lan-dns-settings-card.tsx`** — LAN custom DNS RHF form (`useDNSConfig`, `useSetDNSConfig`). Unused duplicate `dns-config-card.tsx` removed.
- **`lan-dns-preset-buttons.tsx`**, **`lan-dns-server-fields.tsx`** — Preset provider chips + primary/secondary fields (composed by `LanDnsSettingsCard`).
- **`dhcp-dns-card.tsx`** — Composes the two cards (unchanged import for `network-page`).
- **`dhcp-reservations-table.tsx`** — Static reservation rows + delete.
- **`dhcp-reservation-add-form.tsx`** — Add reservation RHF row.
- **`dhcp-reservations-card.tsx`** — Reservations hooks + skeleton + composes table + form.
- **`lib/schemas/network-forms.ts`** — `formatDhcpLeaseTimeHumanLabel` for DHCP lease `Select` labels (used by `dhcp-pool-settings-card.tsx`). Unused duplicate `dhcp-config-card.tsx` removed (pool UI lives under `DhcpDnsCard` / `DhcpPoolSettingsCard`).
- **`data-usage-usage-bar.tsx`** — Monthly budget progress bar (uses shared `formatBytes` from `lib/utils`).
- **`data-usage-interface-card.tsx`** — Per-interface stats + reset + budget bar.
- **`data-usage-budget-editor.tsx`** — Budget GB / warning threshold mini-form.
- **`data-usage-section.tsx`** — Card shell, empty / unavailable states, list composition.

- **`interface-traffic-utils.ts`** — Interface display labels, sort order, rate sampling from WebSocket points.
- **`interface-traffic-chart-card.tsx`** — Single Recharts area card (RX/TX) + latest rates.
- **`interface-traffic-charts.tsx`** — Card shell, WebSocket hook, grid of chart cards.
- **`ddns-status-panel.tsx`** — Running / stopped strip + public IP + last update.
- **`ddns-enabled-fields.tsx`** — Provider select, custom URL, domain, credentials, lookup host.
- **`ddns-card.tsx`** — DDNS hooks, skeleton, form shell.

### VPN page extras (`pages/vpn/`)

- **`vpn-dns-leak-test-card.tsx`** — DNS leak test mutation + result panel.
- **`vpn-verify-wireguard-card.tsx`** — WireGuard verification checks + `StatusRow` helper.
- **`vpn-adguard-hint.tsx`** — Blue info callout when VPN + AdGuard DNS are both on.
- **`vpn-page.tsx`** — Composes WireGuard, split tunnel, hint, verify, leak test, speed test.

### NTP (`pages/system/`)

- **`ntp-config-summary-view.tsx`** — Read-only NTP state + Sync / Edit actions.
- **`ntp-config-server-fields.tsx`** — Server list + add-server draft (`NtpConfigEditForm`).
- **`ntp-config-edit-form.tsx`** — Enable switch, server fields, save / sync / cancel.
- **`ntp-config-card.tsx`** — Hooks, edit mode, composes summary + edit form.

### Forms: React Hook Form + Zod

- **Login** (`pages/login/login-schema.ts` + `login-page.tsx`) uses `useForm` with `zodResolver(loginFormSchema)`; **`login-page-card-header.tsx`**, **`login-form-fields.tsx`** — Branding shell + password / remember / submit.
- **Setup wizard** (`pages/setup/`): `setup-schema.ts` and step components (`welcome-step`, `password-step` + `password-step-intro` / `password-step-form-fields`, `wifi-step` + `wifi-step-intro` / `wifi-step-password-field`, `ap-step` + `ap-step-intro` / `ap-step-credentials-fields`, `complete-step`, `setup-step-indicator`). `setup-page.tsx` orchestrates step index.
- **`setup-wifi-step-utils.ts`** — Signal strength tier for setup scan badges.
- **`setup-wifi-network-list.tsx`** — Scan + scrollable network picker for `WifiStep`.
- **System, network, Wi‑Fi, VPN, clients, logs** — Zod schemas live under `lib/schemas/`; cards use RHF where there is user input. See earlier commits for per-card coverage.

## Assumptions

- **`/services`** highlights “Installed services” only when the path is exactly `/services`; Tailscale under `/services/tailscale` does not activate the parent link.
- **WiFi** sidebar sub-routes: `/wifi` (Wireless), `/wifi/advanced` (Advanced). **Network** sub-routes: `/network` (Status), `/network/configuration`, `/network/advanced`. In-page tab bars stay in sync with these URLs.
- Collapsed desktop sidebar shows a **flat icon list** (one icon per destination), not nested groups.

## Deferred / follow-up (optional)

| Item                                     | Notes                                                                                |
| ---------------------------------------- | ------------------------------------------------------------------------------------ |
| **Accordion vs Collapsible for sidebar** | Collapsible kept for simplicity; Accordion optional for stricter a11y.               |
| **Further splits**                       | Most page modules are ~130 lines or less; extract further only when a feature grows. |
| **Per-card loading UX**                  | Router `Suspense` uses Skeletons; individual cards may still use inline spinners.    |

**Structural refactor (sidebar, pages, router, theme, UI exports, Vite `manualChunks`):** treated as **complete** when `pnpm lint` is clean and `make test` / `make build` pass. The optional rows above are product or UX follow-ups, not pending code splits.

## Component tree (high level)

```
AppShell (useSidebarCollapsed)
├── Sidebar (desktop) | Sheet > Sidebar (mobile)
│   ├── header (title + collapse / close)
│   └── nav (useSidebarGroups)
│       ├── leaf Links
│       └── SidebarNavGroup × N (Collapsible)
├── Header → title, HeaderRouterStatus, HeaderNotificationsMenu, HeaderOverflowMenu, theme
├── OfflineBanner
└── main → Page (lazy) + Suspense + Skeleton

SystemPage
├── SystemAtAGlanceSection
├── Configuration → SystemTimezoneCard, NTPConfigCard, …
├── Maintenance → SystemBackupRestoreCard, FirmwareUpgradeCard
├── SystemPowerSection
└── SystemQuickLinksCard

WireguardSection
├── OperationProgressDialog
└── Card → WireguardCardBody
    ├── WireguardStatusAndConfigPeers
    └── WireguardProfilesKillImport (+ WireguardImportProfileForm)

ClientsPage
├── ClientsConnectedClientsCard
└── ClientsDhcpReservationsCard

WifiPage
├── Tab bar → /wifi | /wifi/advanced
├── Wireless panel → CaptivePortalBanner, WifiModeCard, radios, STA/AP cards
└── Advanced panel → Guest, MAC, policy, band, schedule cards

RepeaterWizard (folder)
├── useRepeaterWizard + Dialog shell (index.tsx)
└── Step components + map-encryption + types
```

---

_When adding features, keep `docs/architecture.md`, `docs/requirements/tasks_open.md`, and `docs/requirements/tasks_done.md` in sync as needed._
