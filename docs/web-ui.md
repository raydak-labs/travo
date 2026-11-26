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
a heading or row spacing. Controls are the copied shadcn-svelte components
([ADR 0013](adr/0013-shadcn-components.md)); a hand-made control needs a measured reason there.

| Component | Use it for |
|---|---|
| `PageHeader` | The top of every page: `title` (the page's one `h1`), optional `description` and `actions` (right of the title from `sm`, below it on phones). |
| `Section` | Each block of the page: one card surface with an `h2` `title`, optional `description`, `actions` (top right of the card) and `footer` (bottom right: the Save of an edit form). There is no other card wrapper. |
| `SettingsRow` | One setting, fact or form field inside a `Section`: `label` and `description` left, the `control` snippet right from `sm`, stacked on phones. Pass `for` when the control has an id. Consecutive rows get a divider by themselves. |
| `LinkRow` | A `SettingsRow` that opens another page: the whole row is the link and ends in a chevron; an optional switch or value sits before the chevron and does not follow the link. |
| `GroupHeading` | The only subheading (`h3`), for a group inside a section or a summary tile. |
| `SummaryBand` / `SummaryTile` | The flat status tiles between the header and the first section. Each tile has its own load fallback, so one failed value never blanks the others. |
| `EmptyState` | "Nothing here yet" content: icon, title, optional text and the add action. A failed load is never empty: `LoadState` shows that. |

### Width

Every page uses the same content column, `max-w-6xl` (1152 px), so the page header and its
actions sit in the same place on every page. Settings cards fill it like tables and grids do;
what stays narrow is the control:

- **Fields** have a fixed width in the row's control slot (`w-28` for numbers and times, `w-64`
  for names, addresses and keys); they never stretch with the card.
- **Cards per row:** related short cards of one topic sit in a `CardGrid`: up to 3 per row from
  `xl`, 2 from `lg`, 1 below. A new topic starts a new row. Long lists, tables and forms with
  many rows stay outside it at full width (DNS names, data usage, interfaces).
- A page whose subject can be switched on or off carries that switch, its state chip and its
  one main action in the **page header** (WireGuard: tunnel; Tailscale: on/off plus Log in or
  Log out; AdGuard Home: running). No card repeats them; cards hold only what adds to it.
- A card whose only control is one switch (all LEDs, night mode) puts the switch in its header
  and the state in its description, instead of a row that repeats the card title.

### Patterns

- **Lists** are a shadcn `Table` inside a `Section`. The add action sits in the section's
  `actions` and opens a `Dialog`. Row actions (edit, activate, remove) are a `⋯`
  `DropdownMenu` at the row end; a row whose only action is remove shows it as a ghost icon
  button with a screen-reader label. A list with no rows shows `EmptyState`. No separate
  "import" or "add" card.
- **Settings that apply on their own** are a `SettingsRow` with a `Switch`, `Select` or short
  field; the change applies at once and reports through `notify` or `transactionToast`. A
  switch already says on or off: no status chip next to it.
- **Edit forms** (several fields saved together: WAN, DHCP server, device settings, LED
  schedule) are `SettingsRow`s with the one Save in the section `footer`. A credential form
  (change password) keeps its fields stacked at `max-w-sm`, as password managers expect.
- **Dialogs** are for adding, editing a list item and confirming (`ConfirmDialog`); never for a
  single toggle or field.
- **Buttons:** the page and section actions are `outline` (with a leading icon for add);
  `default` (filled) is only the submit of a dialog or form; external pages (LuCI, AdGuard
  Home) are `ghost` with a trailing external-link icon; danger actions use `destructive`, sit
  last and still ask through `ConfirmDialog`. Text is never a bare link inside a row: use
  `LinkRow` or a button.
- **Disabled** controls say why next to them ("Connect the tunnel first").

### Status chips

One vocabulary, rectangular `Badge`s (`Status.svelte`), always with text, never colour alone:

| Tone | Means | Examples |
|---|---|---|
| `ok` | working as wanted | Active, Running, Connected, Online |
| `info` | ready or in progress, no action needed | Profile ready, Connecting, 2 updates |
| `warn` | needs attention | Down (configured uplink), Login required, Incomplete |
| `danger` | failed | Error, No handshake, Offline |
| `neutral` | off or not in use | Off, Not set up, Not installed |
| `stale` | last known value, refreshing | Stale |

A chip appears only when it adds something the control next to it does not say.

Scale, applied by these components and the shell:

- Shell padding `p-4`, `md:p-6`.
- The page root is `flex flex-col gap-6`: header, summary band and sections are `gap-6` apart; content inside a section is `gap-4`; a `SettingsRow` is `py-3`.
- `h1` `text-xl font-semibold`; `h2` `text-base font-semibold`; `h3` `text-sm font-medium text-muted-foreground`; descriptions `text-sm text-muted-foreground`.
- Below `sm` (640 px) everything stacks; from `sm` rows and headers sit side by side.

A single-section page gives its section a title that does not repeat the page title.

## Navigation

Sidebar only, every top-level entry with an icon. Everyday pages first:

- Dashboard
- **Internet**: how the router gets online. WiFi networks it joins (saved, scan, join, uplink
  MAC), Ethernet WAN, USB tethering, failover, data usage.
- **WiFi**: the networks the router offers. Access points, guest network, schedule.
- **VPN** ▸ WireGuard · Tailscale
- Clients
- **Services** ▸ one page per installed service that has settings here (AdGuard Home; SQM later)
- **Network** ▸ LAN (address, DHCP server, DNS names) · Port forwards · IPv6 & DDNS · Diagnostics
  (tools, interfaces, firewall zones)
- **System** ▸ Hardware · Logs · Firmware (Travo and OpenWrt updates, firmware upgrade, factory
  reset) · Access

A group label that has an overview page (VPN, Services, System) opens it; the overview is not
repeated as a child. Network has no overview: its label only expands, and `#/network` opens LAN. Old paths
(`#/network/wan`, `#/network/dhcp`, `#/system/updates`, `#/system/reset`, `#/system/leds`,
`#/system/tokens`) open their new page.
The chevron is its own 32 px button and only expands or collapses. A child route expands its
parent. The tree lives in `router.ts` (`NAV_GROUPS`, `GROUP_OVERVIEW`); open groups are stored
under `nav:v2`.

- **VPN overview:** a `LinkRow` per VPN with the current state as hint (WireGuard also carries
  the tunnel switch), then the traffic rules as rows: kill switch (switch), split tunnel (value
  and an Edit dialog), VPN DNS (state and a check).
- **WireGuard:** the profile table with Add profile (dialog: paste or file) and row menus.
- **Tailscale:** status, login and options. Target: off until the user switches it on here (the
  package's post-install start undone by the install flow); needs a daemon on/off action first.
- **Services overview:** installed services first (status and one action pinned to the card
  bottom: Configure, or Open in LuCI for services without a page here), then Available with
  Install. A service page leaves the nav when its package is absent (a failed check is not
  absent).
- **System overview:** a fact strip (model, OpenWrt, uptime, Travo version), the device settings
  form and details. Restart and the app version are only in the header menu.
- **Hardware:** the LEDs and buttons the router really has (read from sysfs and the device
  tree; storage-activity LEDs stay with the kernel), each LED by a readable name with the kernel
  name as hint, one switch for all LEDs, night mode with its dark window, the side switch
  action (applies on change). Per-LED switches need a daemon change first.
- **Access:** API tokens and SSH keys, both as lists; Change password in the header opens a
  dialog.
- **Logs:** one line per entry (time, level chip, process, message cut off; click expands), a
  fixed-height scroll area inside the card, filters that apply while typing, the privacy note.

## Dashboard network map

`TopologyDiagram.svelte` draws `topologyModel()` (`web/src/lib/topology.ts`) in the original
Travo look across the full content width: uplinks (wan, wwan, usbtether) on the left with
label and state before their icon, thin curved links into a square router tile, client counts
on the right. Icons and links share one coordinate system, so every link ends in an icon
centre. Under the router tile: hostname, model and one row of `MapBadge`s: square-cornered
outline badges with a one-word label (Internet, VPN, Tailscale when installed, IPv6 once its
setting is read). The colour is the state: green on, grey off, amber or red only for a problem
(captive portal, offline). The state is in the tooltip and for screen readers, so colour is
never the only signal. Each badge links to its page. DOM and tab order: sources, router, clients.

- Shown from the `lg` breakpoint only; below it the uplink cards, Quick status and System
  cards carry the same facts.
- Hovering or focusing a node opens a `HoverCard` with its details (uplink: protocol, network,
  band, signal, address, gateway, traffic; router: model, uptime, memory; clients: per-band
  counts) and a link to its settings page. From `lg` the uplink cards are not rendered.
- Link per uplink state: active solid `ok`, standby dashed `info`, down and unknown dashed faint
  `neutral`, not set up muted without a link. Every state also has text; nothing animates on
  push. Links are `aria-hidden`; the `figure` carries a hidden text summary.
- Stale is per part: an uplinks or connectivity failure dims the sources, a throughput failure
  only the rates, connectivity or VPN the router, clients the client counts. A failed clients
  read shows Unknown, never 0.

Under the map: Quick status (clients, live traffic, Tailscale, the VPN tunnel with its guarded
toggle) beside the System card (CPU load, memory and storage bars, uptime, the daemon's
low-memory warning), then interface traffic and the history. Below `lg` the uplink cards come
first. The page header holds Connect to WiFi as an outline action.

## Page rules

- Svelte 5 SPA, dark by default without a flash (a stored choice can switch to light), English via Paraglide.
- Size budgets (`web/scripts/size-budget.mjs`): shell ≤ 128 KB gzip, route chunk ≤ 24 KB, total ≤ 320 KB. One chunk per route: a page imports its cards statically ([ADR 0004](adr/0004-frontend-stack.md)).
- Use the copied shadcn-svelte primitives for applicable controls. Keep the app wrapper contracts, semantic tokens and localization; record measured exceptions in [ADR 0013](adr/0013-shadcn-components.md).
- Responsive and mobile-first, route-level code split, vector assets. No CDN or external fonts: the router is often offline.
- Call only the generated API client; no hand-typed endpoints.
- One card surface and one status vocabulary (ok / warn / danger / info / neutral / stale), no raw colour classes.
- One group-heading component and one load/error wrapper: skeleton, inline error with retry, content. A failed request never renders as "absent".
- Multi-status pages have a summary band of flat tiles that each degrade separately.
- `stale` shows the last known value dimmed and marked; never blank a known value.
- One page scroll container. Live charts do not animate on push and state the window they retain. Rates in bits, at most two integer digits, unit per direction.
- Navigation: see [Navigation](#navigation). The header holds actions only: the theme toggle (desktop) and the menu (`HeaderMenu`, loaded on demand) with the app version, theme on phones, Log out and Restart router (the shared `restart.svelte.ts` flow). The logo (`web/src/lib/assets/logo.webp`, favicon `web/public/favicon.png`) is the original Travo logo, used with the operator's permission; it is an image asset, not code, so ADR 0001 does not apply to it. It shows in the sidebar, the phone sheet, and above the login and setup card.
- Errors name the cause. "Rolled back" is shown only when it is true.
