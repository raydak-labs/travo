# shadcn-svelte migration evidence (#119)

Base: `95ecdf0ea9feec6a518be38c84a72272f2ec0166`. After reinstalling the
components with `shadcn-svelte` 1.8.1, budgets are shell 131,072 B, each lazy
JavaScript chunk 16,384 B, total 262,144 B, gzip level 9. The CLI build
measures 69,108 B shell, 10,265 B largest lazy chunk, and 113,836 B total.

Early production probes (before migrating consumers):

| Configuration | Shell B | Largest lazy B | Total B |
| --- | ---: | ---: | ---: |
| Baseline | 37,851 | 5,271 | 58,711 |
| Default upstream Button + Dialog on WiFi route | 63,878 | 19,152 | 98,879 |
| variants/lite, clsx, on-demand Dialog | 44,947 | 15,144 | 80,729 |
| Static variants, explicit lazy library chunks | 39,757 | 6,219 | 77,385 |

The optimized probe still uses the upstream Button template and Bits UI Dialog.
No accessibility functionality is removed. The completed inventory and acceptance
results follow.

Before screenshots: `web/screenshots/before/` (ignored local artifacts), both
390 and 1280 px, light/dark, login/setup/Dashboard/WiFi/VPN/System, WiFi join and
mobile drawer. Captured against the mock daemon on isolated ports 18181/18192.
Preview snapshots failed twice because no screenshot frames were produced;
the repository's headless Chrome helper was used instead.

## Whole-SPA inventory

| Surface | Owned shadcn primitives | Preserved app contract |
| --- | --- | --- |
| `App.svelte`, `ui/MobileNav.svelte` | Button (actions and native-anchor navigation), lazy Sheet | Auth guard, hash navigation/Back, one page scroll container, theme/logout, Close/Escape (Escape while the sheet chunk is pending, Close and Escape if it fails) and Menu focus restoration |
| `ui/Card.svelte` | Card and CardTitle, customized to section/h2 | Optional title and children; existing block layout |
| `ui/Status.svelte`, `ui/status.ts` | Badge | Unchanged ok/warn/danger/info/neutral/stale vocabulary, colors, dot and stale italics |
| `ui/LoadState.svelte`, `ui/FormMessage.svelte` | Skeleton, Alert, Button, Status/Badge | Independent failure/staleness, known data retained, retry/action links |
| `ui/PasswordField.svelte` | Input and Label | Binding, required/password/autocomplete, per-field invalid/describedby relationships |
| Login, Setup | Button and the wrappers above | Validation, busy state, auth/return route and secret handling |
| Dashboard | Button, Table/Header/Body/Row/Head/Cell, wrappers | WiFi navigation button, named interfaces, column/row headers, independent data loaders |
| WiFi, WiFiSummary | Button and wrappers | Summary tiles remain semantic layout; Scan and transaction state logic unchanged |
| WiFiScanList | Collapsible, Button, Status/Badge | Network grouping, per-AP disclosure, disabled unsupported Join, hidden networks and per-radio errors |
| WiFiJoinDialog | Lazy Dialog/Title, Input, Label, Button | Native validation, no outside-click dismissal, Escape/Cancel, secret clearing and trigger focus |
| VPN | Button, Input, Label, Textarea, Alert, wrappers | Import/profile controls, busy/stale states, Pending/Confirm/Roll back and authoritative terminal state |
| System, SystemUpdate | Button, Badge, Alert, wrappers | Password change, backup, inline reboot/update confirmations, guarded operations |
| NotYet, PageHost | Existing wrappers (now composed from shadcn) | Placeholder and retryable lazy loading |
| `ui/GroupHeading.svelte` | Native h3 | A document heading; no interactive group control is currently used |

No applicable interactive primitive remains custom. Native headings, lists,
definition lists, forms, live-region prose, SVG icons and layout containers remain
HTML/SVG as allowed by the brief. WiFiSummary's flat summary tiles are layout,
not a second card component. No unused select, checkbox, menu or tab controls
were introduced. The finite variant lookup and clsx-only class composition are
measured source customizations, not substitute components (see ADR 0013).

The original drawer/backdrop/focus code and native dialog implementation were
removed. The custom skeleton CSS was removed. App wrappers retain their public
interfaces. No Go, API client, router or transaction-state implementation changes.

## Acceptance evidence

Pre-rebase full migration: `mise run check` passed, 225 tests; shell **40,273 B**,
total **82,933 B**, unchanged limits. Rebased onto `84410ce` with Go 1.27.4, Node 26.13.1, Biome 2.6.7, Paraglide
3.2.2, message-format 4.6.0, Lucide 1.72.0 and Vite 8.3.8: `mise run check`
passes again with the same 225 tests and **40,273 B shell / 82,933 B total**.
The largest lazy chunk of that pre-CLI build is **7,493 B** (Bits UI primitives).
The generated API client is unchanged. Browser checks were repeated on this rebased build.

`e2e-components.mjs` passed at 390/1280 px in light/dark themes. It checks real
keyboard Tab/Shift-Tab/Enter/Space/Escape input, visible focus, modal focus loops,
restoration, browser Back, no horizontal overflow, and Dashboard → Scan → Join
Neighbor2 in three activations. Native password length validation rejects short
user input; Escape and Cancel clear it before reopening. Opening/cancelling
Join, reboot or update does not send a mutation. HTTP seams exercise confirmed
Join, VPN Confirm/Roll back, reboot refusal and confirmed update success without
a real device. Existing reducer/transport tests retain terminal-state coverage.

Browser checks cover login/setup, Dashboard, WiFi scan and Join, VPN, System,
reboot/update confirmations and the mobile drawer. Captures stay in the ignored
`web/screenshots/` directory and are not committed. Guarded responses in browser
tests are API test doubles, not claimed device captures. No hardware acceptance
is required for this migration.

First-run `e2e-auth.mjs` also passes after rebase: setup, reload, logout, rejected
password, successful login and return-to-System.

## Self-review correction

Both Standards and Spec reviews identified one minor issue: Menu's
`aria-controls` did not resolve to the mobile Sheet. The new browser assertion
failed before the fix and passes after assigning `id="sidebar"` to Sheet.Content.
The full keyboard/guarded matrix passed again. Sheet's type now advertises only
the supported left side. Measured build after this correction:
**40,274 B shell, 7,489 B largest lazy chunk, 82,927 B total**. After the review
follow-up (dark default, button contrast, drawer dismissal): **40,705 B shell,
7,458 B largest lazy chunk, 82,952 B total**. `mise run check` passes with 225
tests; generated API client remains unchanged.
