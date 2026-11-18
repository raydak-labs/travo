# Web UI patterns: confirmations and results

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
