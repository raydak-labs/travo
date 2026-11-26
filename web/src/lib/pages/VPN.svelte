<script lang="ts">
import { onDestroy } from 'svelte';
import Switch from '$lib/components/ui/switch/switch.svelte';
import '../api/client';
import { getVpnProfiles } from '../api/gen';
import { changeTailscale, fetchTailscale, tailscalePhase } from '../api/tailscale';
import { fetchVpnStatus } from '../api/vpn-status';
import { formatUptime } from '../format';
import { m } from '../paraglide/messages.js';
import { href } from '../router';
import LinkRow from '../ui/LinkRow.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import PageHeader from '../ui/PageHeader.svelte';
import Section from '../ui/Section.svelte';
import Status from '../ui/Status.svelte';
import { createTunnelControl, tunnelStates } from '../vpn-tunnel.svelte';
import { tailscaleStates } from './tailscale-states';
import VPNDNSRow from './VPNDNSRow.svelte';
import VPNKillSwitchRow from './VPNKillSwitchRow.svelte';
import VPNSplitRow from './VPNSplitRow.svelte';
import VPNTunnelSwitch from './VPNTunnelSwitch.svelte';

// The VPN overview: every VPN with its state and switch, then the rules for tunnel traffic.
const status = createLoader(fetchVpnStatus, 10000);
const profiles = createLoader(async (signal) => {
  const result = await getVpnProfiles({ signal });
  if (!result.data) throw new Error(m.vpn_unavailable());
  return result.data;
});
const tailscale = createLoader(fetchTailscale, 10000);
const tunnel = createTunnelControl(() => status.retry());
let tsPending = $state(false);

async function switchTailscale(on: boolean) {
  if (tsPending) return;
  tsPending = true;
  notify.progress(m.tailscale_applying(), 'tailscale');
  const result = await changeTailscale(on ? 'on' : 'off');
  tsPending = false;
  if ('error' in result) notify.error(result.error, 'tailscale');
  else notify.success(m.tailscale_done(), 'tailscale');
  await tailscale.retry();
}
onDestroy(() => {
  status.stop();
  profiles.stop();
  tailscale.stop();
  tunnel.stop();
});

const wireguardHint = $derived.by(() => {
  const snapshot = status.data;
  if (!snapshot) return undefined;
  const state = tunnelStates[snapshot.state].label();
  const profile = profiles.data?.profiles.find((p) => p.id === profiles.data?.selected_id);
  if (!profile) return `${state} · ${m.vpn_no_profile()}`;
  const age =
    snapshot.handshake_age_seconds !== null
      ? ` · ${m.vpn_handshake_age({ age: formatUptime(snapshot.handshake_age_seconds) })}`
      : '';
  return `${state} · ${profile.name}${age}`;
});
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_vpn()} description={m.vpn_page_description()} />

  <Section title={m.vpn_connections()}>
    <div>
      <LinkRow href={href('vpn-wireguard')} label={m.nav_vpn_wireguard()} description={wireguardHint}>
        {#snippet control()}
          <LoadState status={status.status} retry={status.retry} stale={status.data !== undefined}>
            {#if status.data}
              <VPNTunnelSwitch id="vpn-tunnel-switch" snapshot={status.data} control={tunnel} canEnable={Boolean(profiles.data?.selected_id)} />
            {/if}
          </LoadState>
        {/snippet}
      </LinkRow>
      <LinkRow
        href={href('vpn-tailscale')}
        label={m.nav_vpn_tailscale()}
        description={m.tailscale_vpn_row()}
      >
        {#snippet control()}
          {#if tailscale.data}
            {@const st = tailscale.data}
            {@const state = tailscaleStates[tailscalePhase(st)]}
            {#if st.running}<Status kind={state.kind}>{state.label()}</Status>{/if}
            {#if st.installed}
              <Switch id="vpn-tailscale-switch" aria-label={m.tailscale_running_label()} disabled={tsPending || st.busy}
                bind:checked={() => st.running, (on) => void switchTailscale(on)} />
            {:else}
              <Status kind="neutral">{state.label()}</Status>
            {/if}
          {/if}
        {/snippet}
      </LinkRow>
    </div>
  </Section>

  <Section title={m.vpn_traffic_rules()}>
    <div>
      <VPNKillSwitchRow tunnel={status.data} />
      <VPNSplitRow />
      <VPNDNSRow />
    </div>
  </Section>
</div>
