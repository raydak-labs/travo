<script lang="ts">
import type { Snippet } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import type { TunnelStatus } from '../api/gen';
import { formatUptime } from '../format';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import type { LoadStatus } from '../ui/loader.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import Status from '../ui/Status.svelte';
import { transactionToast } from '../ui/transaction-toast.svelte';
import {
  controlPhase,
  type TunnelControl,
  tunnelEnabled,
  tunnelStates,
} from '../vpn-tunnel.svelte';

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

transactionToast(
  'vpn-tunnel',
  () => controlPhase(control, m.vpn_tunnel_applying()),
  (ok) => void control.settle(ok),
);
const confirm = createConfirm();

// Turning the tunnel off sends LAN traffic past the VPN (or cuts it with the kill switch).
async function toggle(enabled: boolean) {
  if (enabled) {
    const ok = await confirm.ask({
      title: m.vpn_tunnel_disable_title(),
      description: m.vpn_tunnel_disable_body(),
      action: m.vpn_tunnel_disable(),
      tone: 'danger',
    });
    if (!ok) return;
  }
  void control.set(!enabled);
}
</script>

<LoadState status={status.status} retry={status.retry} stale={status.data !== undefined}>
  {#if status.data}
    {@const snapshot = status.data}
    {@const badge = tunnelStates[snapshot.state]}
    <SettingsRow label={m.vpn_status_label()}
      description={snapshot.handshake_age_seconds !== null ? m.vpn_handshake_age({ age: formatUptime(snapshot.handshake_age_seconds) }) : undefined}>
      {#snippet control()}
        <Status kind={badge.kind}>{badge.label()}</Status>
        {#if snapshot.stale}<Status kind="stale">{m.load_stale()}</Status>{/if}
      {/snippet}
    </SettingsRow>
  {/if}
</LoadState>
{#if status.data && !control.busy}
  {@const snapshot = status.data}
  {@const enabled = tunnelEnabled(snapshot.state)}
  <div class="flex flex-wrap gap-2">
    <!-- Turning the tunnel off is the danger action, so it sits last. -->
    {@render extra?.(snapshot)}
    <Button variant={enabled ? 'destructive' : 'outline'} size="sm" type="button" disabled={snapshot.stale || (!enabled && !canEnable(snapshot))} onclick={() => toggle(enabled)}>{enabled ? m.vpn_tunnel_disable() : m.vpn_tunnel_enable()}</Button>
  </div>
  {#if !enabled && !canEnable(snapshot)}
    <p class="text-sm text-muted-foreground">{m.vpn_tunnel_needs_profile()}</p>
  {/if}
{/if}

<ConfirmDialog {confirm} />
