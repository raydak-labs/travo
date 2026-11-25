# Web UI patterns

Every page uses the same three tools from `web/src/lib/ui/`. A new page follows them
instead of putting warnings, confirm buttons or result text at the bottom of a form.

## Ask before a risky change: `ConfirmDialog`

For a change that can cut the user off, drop clients or lose data (reboot, update,
lockout override, disabling an AP or the tunnel, kill switch, forgetting a saved
uplink, deleting a profile, the captive DNS bypass):

```svelte
<script lang="ts">
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';

const confirm = createConfirm();
async function restart() {
  const ok = await confirm.ask({
    title: m.reboot_confirm_title(),
    description: m.reboot_confirm(),
    action: m.reboot_router(), // the button names the action, never "OK"
    tone: 'danger',
  });
  if (ok) void send();
}
</script>

<ConfirmDialog {confirm} />
```

`ask` resolves `false` on Cancel, Escape or a newer question. The dialog is the
shadcn-svelte AlertDialog; Cancel is focused first. A daemon refusal that the user may
override (`wifi_lockout_risk`) opens the same dialog and resends with the acknowledge flag
only on the action.

## Guarded transactions: `transactionToast`

A change that runs through the transaction module (WiFi join and AP edits, VPN tunnel,
kill switch, split tunnel) maps its state to a `TxnPhase` and mirrors it into one toast:

```ts
transactionToast('vpn-killswitch', () => controlPhase(control, m.vpn_kill_applying()),
  (ok) => void control.settle(ok));
```

The toast shows progress, then the confirm window with a countdown and Keep change /
Roll back, then the outcome (confirmed, rolled back, rollback failed, refused). Open
phases cannot be swiped away and close with their page; errors stay until dismissed.
Write a pure `…Phase` mapper for a new flow (see `joinPhase`, `controlPhase`) and test it.

## Other results: `notify`

Saved, deleted, imported, failed: `notify.success(text)`, `notify.error(text)`. A long
action shows `notify.progress(text, id)` and finishes with `notify.success/error(text, id)`.
Field validation stays next to its field; long-lived state (a failed load, AdGuard
fail-open, an update in progress) stays in the card.

The shell mounts the one `Toaster` (svelte-sonner) in `App.svelte`. `mise run screenshots`
also captures a dialog, the pending toast and a result toast.

## Layout

Pages are built from the wrappers in `web/src/lib/ui/`; a page never hand-rolls a card,
a heading or row spacing.

| Component | Use it for |
|---|---|
| `PageHeader` | The top of every page: `title` (the page's one `h1`), optional `description` and `actions` (right of the title from `sm`, below it on phones). |
| `Section` | Each block of the page: one card surface with an `h2` `title`, optional `description` and `actions` (the card action slot, e.g. Scan). There is no other card wrapper. |
| `SettingsRow` | One setting or item inside a `Section`: `label` and `description` left, the `control` snippet right from `sm`, stacked on phones. Pass `for` when the control has an id so the label targets it. Consecutive rows get a divider by themselves. |
| `GroupHeading` | The only subheading (`h3`), for a group inside a section (Guest network) or a summary tile. |
| `SummaryBand` / `SummaryTile` | The flat status tiles between the header and the first section (WiFi status, Dashboard resources). The band carries a visually hidden `h2`; each tile has a `GroupHeading` label and its own load fallback, so one failed value never blanks the others. |
| `EmptyState` | "Nothing here yet" content: icon, title, optional text and action. A failed load is never empty: `LoadState` shows that. |

Scale, applied by these components and the shell:

- Shell padding `p-4`, `md:p-6`; the content column stays `max-w-3xl`.
- The page root is `flex flex-col gap-6`: header, summary band and sections are `gap-6` apart; content inside a section is `gap-4`; a `SettingsRow` is `py-3`.
- `h1` `text-xl font-semibold`; `h2` `text-base font-semibold`; `h3` `text-sm font-medium text-muted-foreground`; descriptions `text-sm text-muted-foreground`.
- Below `sm` (640 px) everything stacks; from `sm` rows and headers sit side by side.
- A summary band sits between the header (or the Dashboard's network map) and the next section, outside any card; `SummaryBand` keeps the heading order h1 → h2 → h3.

The WiFi page is the reference that the other pages copy.

Edit forms (several fields saved together with one button, such as WAN, DHCP server, device settings or the LED schedule) stay stacked label-over-input inside their `Section`; `SettingsRow` is for a setting that applies on its own and for list items. A single-section page gives its section a title that does not repeat the page title. A control that is disabled says why next to it.

Danger actions (disable, delete, remove, revoke, restart, block) use the `destructive` button tone and sit last in their row or section; they still ask through `ConfirmDialog`.

## Dashboard network map

`TopologyDiagram.svelte` draws `topologyModel()` (`web/src/lib/topology.ts`) in the original Travo look: uplinks (wan, wwan, usbtether) as light rows (status dot, icon, label, state and detail, live rate) on the left, thin curved links into a square router tile with a status dot, hostname, model and Internet/VPN/Tailscale chips, and client counts (WiFi, wired) as large numbers on the right. DOM and tab order: sources, router, clients.

- Shown from the `lg` breakpoint only; below it the dashboard cards carry the same facts.
- Link per uplink state: active solid `ok`, standby dashed `info`, down and unknown dashed faint `neutral`, not set up dimmed without a link. Every state also has text; nothing animates on push. Links are `aria-hidden`; the `figure` carries a hidden text summary.
- Stale is per part: an uplinks or connectivity failure dims the sources, a throughput failure only the rates, connectivity or VPN the router, clients the client counts. A failed clients read shows Unknown, never 0.

Under the map: one card per uplink (state; WAN protocol; WiFi network, band and signal; USB tether kind; address; gateway on the uplink that carries the default route; device; live traffic; "Set up" when not configured), Quick status (clients, live traffic of the active uplink, Tailscale, the VPN tunnel with its guarded toggle) and the System card (CPU load, memory and storage bars, uptime, the daemon's low-memory warning). The app version is in the header menu.

## Page rules

- Svelte 5 SPA, dark by default without a flash (a stored choice can switch to light), English via Paraglide.
- Size budgets (`web/scripts/size-budget.mjs`): shell ≤ 128 KB gzip, route chunk ≤ 24 KB, total ≤ 320 KB.
- Use the copied shadcn-svelte primitives for applicable controls. Keep the app wrapper contracts, semantic tokens and localization; record measured exceptions in [ADR 0013](adr/0013-shadcn-components.md).
- Responsive and mobile-first, route-level code split, vector assets. No CDN or external fonts: the router is often offline.
- Call only the generated API client; no hand-typed endpoints.
- One card surface and one status vocabulary (ok / warn / danger / info / neutral / stale), no raw colour classes.
- One group-heading component and one load/error wrapper: skeleton, inline error with retry, content. A failed request never renders as "absent".
- Multi-status pages have a summary band of flat tiles that each degrade separately (WiFi is the reference).
- `stale` shows the last known value dimmed and marked; never blank a known value.
- One page scroll container. Live charts do not animate on push and state the window they retain. Rates in bits, at most two integer digits, unit per direction.
- Sidebar-only nav. Everyday pages first: Dashboard, Internet, WiFi, VPN & Tailscale, Clients, Services. Expert settings sit one level deeper in two groups: Network (WAN, LAN, DHCP & DNS names, Port forwards, IPv6 & DDNS, Diagnostics; the label opens WAN) and System (Device, LEDs & switch, Logs, Updates & firmware, API tokens, Factory reset; the label opens Device). Each child is its own route and page chunk; `#/network` opens WAN. The tree lives in `router.ts` (`NAV_GROUPS`) and `App.svelte`; open groups are stored under `nav:v1`. The header holds actions only: the theme toggle (desktop) and the menu (`HeaderMenu`, loaded on demand) with the app version, theme on phones, Log out and Restart router (the shared `restart.svelte.ts` flow, also used by System → Device). The logo (`web/src/lib/assets/logo.webp`, favicon `web/public/favicon.png`) is the original Travo logo, used with the operator's permission; it is an image asset, not code, so ADR 0001 does not apply to it. It shows in the sidebar, the phone sheet, and above the login and setup card. A first visit lands on Dashboard with groups collapsed; a child route expands its parent; the group label opens the default page and the chevron toggles; stored nav state is versioned.
- Optional service pages leave the nav when their package is absent (a failed check is not absent).
- Errors name the cause. "Rolled back" is shown only when it is true.
