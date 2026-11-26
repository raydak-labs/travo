<script lang="ts">
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import * as Dialog from '$lib/components/ui/dialog';
import Label from '$lib/components/ui/label/label.svelte';
import Textarea from '$lib/components/ui/textarea/textarea.svelte';
import '../api/client';
import { setVpnSplit } from '../api/gen';
import { fetchSplit } from '../api/vpn-status';
import { m } from '../paraglide/messages.js';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import { transactionToast } from '../ui/transaction-toast.svelte';
import { controlPhase, createTunnelControl, parseRoutes, splitRefusal } from '../vpn-tunnel.svelte';

const split = createLoader(fetchSplit, 10000);
const formId = $props.id();
let open = $state(false);
let draft = $state<string>();
const text = $derived(draft ?? split.data?.routes.join('\n') ?? '');
// The shared toggle submits one change; `enabled` is unused: the routes come from the draft.
const txn = createTunnelControl(
  () => {
    draft = undefined;
    return split.retry();
  },
  {
    submit: (_enabled, signal) => setVpnSplit({ body: { routes: parseRoutes(text) }, signal }),
    refusal: splitRefusal,
    confirmed: m.vpn_split_confirmed,
    rolledBack: m.vpn_split_rolled_back,
    rollbackFailed: m.vpn_tunnel_rollback_failed,
  },
);
transactionToast(
  'vpn-split',
  () => controlPhase(txn, m.vpn_tunnel_applying()),
  (ok) => void txn.settle(ok),
);
onDestroy(() => {
  split.stop();
  txn.stop();
});
</script>

<!-- One row in the VPN page's traffic rules; the routes are edited in a dialog. -->
<SettingsRow label={m.vpn_split()} description={m.vpn_split_help()}>
  {#snippet below()}
    {#if split.status.state !== 'ready'}
      <LoadState status={split.status} retry={split.retry} stale={split.data !== undefined}><span></span></LoadState>
    {/if}
  {/snippet}
  {#snippet control()}
    {#if split.data}
      <span class="text-sm text-muted-foreground {split.status.state !== 'ready' ? 'opacity-60' : ''}">{split.data.routes.length ? m.vpn_split_on({ count: split.data.routes.length }) : m.vpn_split_full()}</span>
      <Button variant="outline" size="sm" type="button" disabled={txn.busy} onclick={() => { draft = undefined; open = true; }}>{m.vpn_split_edit()}</Button>
    {/if}
  {/snippet}
</SettingsRow>

<Dialog.Root bind:open>
  <Dialog.Content portalProps={{ disabled: typeof window === 'undefined' }}>
    <Dialog.Header>
      <Dialog.Title level={2}>{m.vpn_split()}</Dialog.Title>
      <Dialog.Description>{m.vpn_split_help()}</Dialog.Description>
    </Dialog.Header>
    <form id={formId} class="grid gap-2" onsubmit={(event) => { event.preventDefault(); open = false; void txn.set(true); }}>
      <Label for="{formId}-routes">{m.vpn_split_routes()}</Label>
      <Textarea id="{formId}-routes" class="w-full font-mono" rows={6} value={text} oninput={(event) => { draft = event.currentTarget.value; }} spellcheck="false" placeholder="10.0.0.0/8"></Textarea>
    </form>
    <Dialog.Footer>
      <Button variant="outline" type="button" onclick={() => { open = false; }}>{m.dialog_cancel()}</Button>
      <Button type="submit" form={formId}>{m.vpn_split_save()}</Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>
