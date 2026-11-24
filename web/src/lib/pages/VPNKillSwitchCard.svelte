<script lang="ts">
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import '../api/client';
import { setVpnKillSwitch, type TunnelStatus } from '../api/gen';
import { fetchKillSwitch } from '../api/vpn-status';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import Section from '../ui/Section.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import Status from '../ui/Status.svelte';
import { transactionToast } from '../ui/transaction-toast.svelte';
import { controlPhase, createTunnelControl, killSwitchRefusal } from '../vpn-tunnel.svelte';

let { tunnel }: { tunnel?: TunnelStatus } = $props();

const state = createLoader(fetchKillSwitch, 10000);
const control = createTunnelControl(() => state.retry(), {
  submit: (enabled, signal) => setVpnKillSwitch({ body: { enabled }, signal }),
  refusal: killSwitchRefusal,
  confirmed: m.vpn_kill_confirmed,
  rolledBack: m.vpn_kill_rolled_back,
  rollbackFailed: m.vpn_kill_rollback_failed,
});
transactionToast(
  'vpn-killswitch',
  () => controlPhase(control, m.vpn_kill_applying()),
  (ok) => void control.settle(ok),
);
onDestroy(() => {
  state.stop();
  control.stop();
});
const connected = $derived(tunnel?.state === 'connected' && !tunnel.stale);
const confirm = createConfirm();

// Both directions change who reaches the internet: on cuts LAN off while the tunnel is
// down, off lets LAN fall back to the uplink.
async function change(enable: boolean) {
  const ok = await confirm.ask({
    title: enable ? m.vpn_kill_enable_title() : m.vpn_kill_disable_title(),
    description: enable ? m.vpn_kill_on_help() : m.vpn_kill_off_help(),
    action: enable ? m.vpn_kill_enable() : m.vpn_kill_disable(),
    tone: enable ? 'default' : 'danger',
  });
  if (ok) void control.set(enable);
}
</script>

<Section title={m.vpn_kill()} description={state.data ? (state.data.enabled ? m.vpn_kill_on_help() : m.vpn_kill_off_help()) : undefined}>
  <LoadState status={state.status} retry={state.retry} stale={state.data !== undefined}>
    {#if state.data}
      {@const on = state.data.enabled}
      <SettingsRow label={m.vpn_status_label()}>
        {#snippet control()}
          <Status kind={on ? 'ok' : 'neutral'}>{on ? m.vpn_kill_on() : m.vpn_kill_off()}</Status>
        {/snippet}
      </SettingsRow>
    {/if}
  </LoadState>
  {#if state.data && !control.busy}
    <div class="flex flex-wrap gap-2">
      {#if state.data.enabled}
        <Button variant="destructive" size="sm" type="button" onclick={() => change(false)}>{m.vpn_kill_disable()}</Button>
      {:else}
        <Button variant="outline" size="sm" type="button" disabled={!connected} onclick={() => change(true)}>{m.vpn_kill_enable()}</Button>
      {/if}
    </div>
    {#if !state.data.enabled && !connected}
      <p class="text-sm text-muted-foreground">{m.vpn_kill_needs_tunnel()}</p>
    {/if}
  {/if}
</Section>

<ConfirmDialog {confirm} />
