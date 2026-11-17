<script lang="ts">
import { onDestroy } from 'svelte';
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import Textarea from '$lib/components/ui/textarea/textarea.svelte';
import '../api/client';
import { setVpnSplit } from '../api/gen';
import { fetchSplit } from '../api/vpn-status';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import Status from '../ui/Status.svelte';
import { createTunnelControl, parseRoutes, splitRefusal } from '../vpn-tunnel.svelte';

const split = createLoader(fetchSplit, 10000);
let draft = $state<string>();
const text = $derived(draft ?? split.data?.routes.join('\n') ?? '');
// The shared toggle submits one change; `enabled` is unused: the routes come from the draft.
const control = createTunnelControl(
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
onDestroy(() => {
  split.stop();
  control.stop();
});
</script>

<Card title={m.vpn_split()}>
  <LoadState status={split.status} retry={split.retry} stale={split.data !== undefined}>
    {#if split.data}
      <Status kind="info">{split.data.routes.length ? m.vpn_split_on({ count: split.data.routes.length }) : m.vpn_split_full()}</Status>
    {/if}
  </LoadState>
  <form class="mt-3 grid gap-3" onsubmit={(event) => { event.preventDefault(); void control.set(true); }}>
    <Label class="grid gap-1">
      <span>{m.vpn_split_routes()}</span>
      <Textarea class="w-full font-mono" rows={4} value={text} oninput={(event) => { draft = event.currentTarget.value; }} disabled={control.busy} spellcheck="false" placeholder="10.0.0.0/8"></Textarea>
    </Label>
    <p class="text-muted-foreground">{m.vpn_split_help()}</p>
    <div class="flex flex-wrap gap-2">
      {#if control.transaction?.state === 'pending'}
        <Button variant="outline" size="sm" type="button" disabled={control.settling} onclick={() => control.settle(true)}>{m.vpn_tunnel_confirm()}</Button>
        <Button variant="outline" size="sm" type="button" disabled={control.settling} onclick={() => control.settle(false)}>{m.vpn_tunnel_rollback()}</Button>
      {:else}
        <Button variant="outline" size="sm" type="submit" class="w-fit" disabled={control.busy || !split.data}>{m.vpn_split_save()}</Button>
      {/if}
    </div>
  </form>
  {#if control.applying}
    <p role="status" class="mt-2">{m.vpn_tunnel_applying()}</p>
  {:else if control.transaction?.state === 'pending'}
    <p role="status" class="mt-2">{m.vpn_tunnel_pending()}</p>
  {:else if control.transaction?.state === 'restoring'}
    <p role="status" class="mt-2">{m.vpn_tunnel_restoring()}</p>
  {:else if control.message}
    <p role="status" class="mt-2">{control.message}</p>
  {/if}
  {#if control.error}<Alert class="mt-2 text-danger">{control.error}</Alert>{/if}
</Card>
