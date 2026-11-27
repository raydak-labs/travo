<script lang="ts">
import type { TailscaleStatus, TunnelStatus } from '../api/gen';
import { tailscalePhase } from '../api/tailscale';
import { formatBitRate } from '../format';
import { m } from '../paraglide/messages.js';
import { href } from '../router';
import type { TopologyModel } from '../topology';
import { internetChip } from '../topology-labels';
import LinkRow from '../ui/LinkRow.svelte';
import LoadState from '../ui/LoadState.svelte';
import type { LoadStatus } from '../ui/loader.svelte';
import Section from '../ui/Section.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import Status from '../ui/Status.svelte';
import { type TunnelControl, tunnelStates } from '../vpn-tunnel.svelte';
import TailscaleSwitch from './TailscaleSwitch.svelte';
import { tailscaleStates } from './tailscale-states';
import VPNTunnelSwitch from './VPNTunnelSwitch.svelte';

// The things people check and toggle most: clients, live traffic, Tailscale, the VPN tunnel.
type Part = 'connectivity' | 'clients' | 'throughput';
let {
  model,
  status,
  vpn,
  tunnel,
  tailscale,
}: {
  model: TopologyModel;
  status: Record<Part, LoadStatus>;
  vpn: { status: LoadStatus; data?: TunnelStatus; retry: () => void };
  tunnel: TunnelControl;
  tailscale: { status: LoadStatus; data?: TailscaleStatus; retry: () => void };
} = $props();

const active = $derived(model.sources.find((s) => s.state === 'active'));
// Stale per row: a known value whose source is reloading or failed stays, dimmed and marked.
const stale = (part: Part, known: boolean) => known && status[part].state !== 'ready';
const tailscaleDescription = $derived(
  tailscale.data ? tailscaleStates[tailscalePhase(tailscale.data)].label() : undefined,
);
</script>

{#snippet staleMark(on: boolean)}
  {#if on}<Status kind="stale">{m.load_stale()}</Status>{/if}
{/snippet}

<Section title={m.dashboard_quick_status()}>
  <div>
    <SettingsRow label={m.topology_internet()}>
      {#snippet control()}
        {@const chip = internetChip[model.internet]}
        {@const old = stale('connectivity', model.internet !== 'unknown')}
        <span class={old ? 'opacity-60' : ''}><Status kind={chip.kind}>{chip.label()}</Status></span>
        {@render staleMark(old)}
      {/snippet}
    </SettingsRow>
    <SettingsRow label={m.nav_clients()}>
      {#snippet control()}
        {@const old = stale('clients', model.clients !== null)}
        <a href={href('clients')} class="tabular-nums underline-offset-4 hover:underline {old ? 'opacity-60' : ''}">
          {model.clients
            ? `${m.topology_clients_wifi({ count: model.clients.wifi })} · ${m.topology_clients_lan({ count: model.clients.lan })}`
            : m.topology_state_unknown()}
        </a>
        {#if model.clients?.partial}<Status kind="warn">{m.topology_clients_incomplete()}</Status>{/if}
        {@render staleMark(old)}
      {/snippet}
    </SettingsRow>
    <SettingsRow label={m.uplink_card_traffic()}>
      {#snippet control()}
        {@const old = stale('throughput', active?.rate != null)}
        <span class="text-sm tabular-nums {old ? 'opacity-60' : ''}">
          {#if active?.rate}
            ↓ {formatBitRate(active.rate.rxBitsPerSecond)} · ↑ {formatBitRate(active.rate.txBitsPerSecond)}
          {:else}
            {m.dashboard_rate_unknown()}
          {/if}
        </span>
        {@render staleMark(old)}
      {/snippet}
    </SettingsRow>
    {#if model.tailscale !== 'absent'}
      <LinkRow href={href('vpn-tailscale')} label={m.tailscale_title()} description={tailscaleDescription}>
        {#snippet control()}
          <LoadState status={tailscale.status} retry={tailscale.retry} stale={tailscale.data !== undefined}>
            {#if tailscale.data?.installed}
              <TailscaleSwitch id="dashboard-tailscale-switch" status={tailscale.data}
                disabled={tailscale.status.state !== 'ready'} refresh={tailscale.retry} />
            {/if}
          </LoadState>
        {/snippet}
      </LinkRow>
    {/if}
    <LinkRow
      href={href('vpn')}
      label={m.dashboard_vpn()}
      description={vpn.data ? (vpn.data.state === 'disabled' ? `${tunnelStates.disabled.label()} · ${m.vpn_no_profile()}` : tunnelStates[vpn.data.state].label()) : undefined}
    >
      {#snippet control()}
        <LoadState status={vpn.status} retry={vpn.retry} stale={vpn.data !== undefined}>
          {#if vpn.data}
            <VPNTunnelSwitch id="dashboard-vpn-switch" snapshot={vpn.data} control={tunnel}
              canEnable={vpn.data.state === 'configured'} />
          {/if}
        </LoadState>
      {/snippet}
    </LinkRow>
  </div>
</Section>
