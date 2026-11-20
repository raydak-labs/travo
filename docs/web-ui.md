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

## Page rules

- Svelte 5 SPA, dark by default without a flash (a stored choice can switch to light), English via Paraglide.
- Size budgets (`web/scripts/size-budget.mjs`): shell ≤ 128 KB gzip, route chunk ≤ 16 KB, total ≤ 256 KB.
- Use the copied shadcn-svelte primitives for applicable controls. Keep the app wrapper contracts, semantic tokens and localization; record measured exceptions in [ADR 0013](adr/0013-shadcn-components.md).
- Responsive and mobile-first, route-level code split, vector assets. No CDN or external fonts: the router is often offline.
- Call only the generated API client; no hand-typed endpoints.
- One card surface and one status vocabulary (ok / warn / danger / info / neutral / stale), no raw colour classes.
- One group-heading component and one load/error wrapper: skeleton, inline error with retry, content. A failed request never renders as "absent".
- Multi-status pages have a summary band of flat tiles that each degrade separately (WiFi is the reference).
- `stale` shows the last known value dimmed and marked; never blank a known value.
- One page scroll container. Live charts do not animate on push and state the window they retain. Rates in bits, at most two integer digits, unit per direction.
- Sidebar-only nav: Dashboard, WiFi, Network, VPN, Services, System. The header holds actions only. A first visit lands on Dashboard with groups collapsed; a child route expands its parent; the group label opens the default page and the chevron toggles; stored nav state is versioned.
- Optional service pages leave the nav when their package is absent (a failed check is not absent).
- Errors name the cause. "Rolled back" is shown only when it is true.
