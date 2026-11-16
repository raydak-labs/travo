<script lang="ts">
import type { Snippet } from 'svelte';
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import type { TunnelStatus } from '../api/gen';
import { formatUptime } from '../format';
import { m } from '../paraglide/messages.js';
import LoadState from '../ui/LoadState.svelte';
import type { LoadStatus } from '../ui/loader.svelte';
import Status from '../ui/Status.svelte';
import { type TunnelControl, tunnelEnabled, tunnelStates } from '../vpn-tunnel.svelte';

// Shared by the VPN page and the dashboard quick action so both show the same states.
let {
  status,
  control,
  canEnable,
  extra,
}: {
  status: { status: LoadStatus; data?: TunnelStatus; retry: () => void };
  control: TunnelControl;
  /** Whether a profile is selected; `configured` already implies one. */
  canEnable: (snapshot: TunnelStatus) => boolean;
  /** Extra actions rendered next to the toggle while no transaction is open. */
  extra?: Snippet<[TunnelStatus]>;
} = $props();
</script>

<LoadState status={status.status} retry={status.retry} stale={status.data !== undefined}>
  {#if status.data}
    {@const snapshot = status.data}
    {@const badge = tunnelStates[snapshot.state]}
    <div class="flex flex-wrap items-center gap-2">
      <Status kind={badge.kind}>{badge.label()}</Status>
      {#if snapshot.stale}<Status kind="stale">{m.load_stale()}</Status>{/if}
    </div>
    {#if snapshot.handshake_age_seconds !== null}
      <p class="mt-2 text-muted-foreground">{m.vpn_handshake_age({ age: formatUptime(snapshot.handshake_age_seconds) })}</p>
    {/if}
  {/if}
</LoadState>
<div class="mt-3 flex flex-wrap gap-2">
  {#if control.transaction?.state === 'pending'}
    <Button variant="outline" size="sm" type="button" disabled={control.settling} onclick={() => control.settle(true)}>{m.vpn_tunnel_confirm()}</Button>
    <Button variant="outline" size="sm" type="button" disabled={control.settling} onclick={() => control.settle(false)}>{m.vpn_tunnel_rollback()}</Button>
  {:else if status.data && !control.busy}
    {@const snapshot = status.data}
    {@const enabled = tunnelEnabled(snapshot.state)}
    <Button variant="outline" size="sm" type="button" disabled={snapshot.stale || (!enabled && !canEnable(snapshot))} onclick={() => control.set(!enabled)}>{enabled ? m.vpn_tunnel_disable() : m.vpn_tunnel_enable()}</Button>
    {@render extra?.(snapshot)}
  {/if}
</div>
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
