<script lang="ts">
import { onDestroy } from 'svelte';
import Switch from '$lib/components/ui/switch/switch.svelte';
import '../api/client';
import { setVpnKillSwitch, type TunnelStatus } from '../api/gen';
import { fetchKillSwitch } from '../api/vpn-status';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import { transactionToast } from '../ui/transaction-toast.svelte';
import { controlPhase, createTunnelControl, killSwitchRefusal } from '../vpn-tunnel.svelte';

let { tunnel }: { tunnel?: TunnelStatus } = $props();

const state = createLoader(fetchKillSwitch, 10000);
const txn = createTunnelControl(() => state.retry(), {
  submit: (enabled, signal) => setVpnKillSwitch({ body: { enabled }, signal }),
  refusal: killSwitchRefusal,
  confirmed: m.vpn_kill_confirmed,
  rolledBack: m.vpn_kill_rolled_back,
  rollbackFailed: m.vpn_kill_rollback_failed,
});
transactionToast(
  'vpn-killswitch',
  () => controlPhase(txn, m.vpn_kill_applying()),
  (ok) => void txn.settle(ok),
);
onDestroy(() => {
  state.stop();
  txn.stop();
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
  if (ok) void txn.set(enable);
}
</script>

<!-- One row in the VPN page's traffic rules; the switch confirms both directions. -->
<SettingsRow
  label={m.vpn_kill()}
  for="vpn-kill-switch"
  description={state.data && !state.data.enabled && !connected
    ? m.vpn_kill_needs_tunnel()
    : state.data?.enabled
      ? m.vpn_kill_on_help()
      : m.vpn_kill_off_help()}
>
  {#snippet control()}
    <LoadState status={state.status} retry={state.retry} stale={state.data !== undefined}>
      {#if state.data}
        {@const on = state.data.enabled}
        <Switch
          id="vpn-kill-switch"
          bind:checked={() => on, (value) => void change(value)}
          disabled={txn.busy || (!on && !connected)}
        />
      {/if}
    </LoadState>
  {/snippet}
</SettingsRow>

<ConfirmDialog {confirm} />
