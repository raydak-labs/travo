<script lang="ts">
import { onDestroy } from 'svelte';
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import '../api/client';
import { setVpnKillSwitch, type TunnelStatus } from '../api/gen';
import { fetchKillSwitch } from '../api/vpn-status';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import Status from '../ui/Status.svelte';
import { createTunnelControl, killSwitchRefusal } from '../vpn-tunnel.svelte';

let { tunnel }: { tunnel?: TunnelStatus } = $props();

const state = createLoader(fetchKillSwitch, 10000);
const control = createTunnelControl(() => state.retry(), {
  submit: (enabled, signal) => setVpnKillSwitch({ body: { enabled }, signal }),
  refusal: killSwitchRefusal,
  confirmed: m.vpn_kill_confirmed,
  rolledBack: m.vpn_kill_rolled_back,
  rollbackFailed: m.vpn_kill_rollback_failed,
});
onDestroy(() => {
  state.stop();
  control.stop();
});
const connected = $derived(tunnel?.state === 'connected' && !tunnel.stale);
</script>

<Card title={m.vpn_kill()}>
  <LoadState status={state.status} retry={state.retry} stale={state.data !== undefined}>
    {#if state.data}
      <Status kind={state.data.enabled ? 'ok' : 'neutral'}>{state.data.enabled ? m.vpn_kill_on() : m.vpn_kill_off()}</Status>
      <p class="mt-2 text-muted-foreground">{state.data.enabled ? m.vpn_kill_on_help() : m.vpn_kill_off_help()}</p>
    {/if}
  </LoadState>
  <div class="mt-3 flex flex-wrap gap-2">
    {#if control.transaction?.state === 'pending'}
      <Button variant="outline" size="sm" type="button" disabled={control.settling} onclick={() => control.settle(true)}>{m.vpn_tunnel_confirm()}</Button>
      <Button variant="outline" size="sm" type="button" disabled={control.settling} onclick={() => control.settle(false)}>{m.vpn_tunnel_rollback()}</Button>
    {:else if state.data && !control.busy}
      {#if state.data.enabled}
        <Button variant="outline" size="sm" type="button" onclick={() => control.set(false)}>{m.vpn_kill_disable()}</Button>
      {:else}
        <Button variant="outline" size="sm" type="button" disabled={!connected} onclick={() => control.set(true)}>{m.vpn_kill_enable()}</Button>
      {/if}
    {/if}
  </div>
  {#if control.applying}
    <p role="status" class="mt-2">{m.vpn_kill_applying()}</p>
  {:else if control.transaction?.state === 'pending'}
    <p role="status" class="mt-2">{m.vpn_tunnel_pending()}</p>
  {:else if control.transaction?.state === 'restoring'}
    <p role="status" class="mt-2">{m.vpn_tunnel_restoring()}</p>
  {:else if control.message}
    <p role="status" class="mt-2">{control.message}</p>
  {/if}
  {#if control.error}<Alert class="mt-2 text-danger">{control.error}</Alert>{/if}
</Card>
