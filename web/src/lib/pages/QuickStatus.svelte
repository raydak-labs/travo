<script lang="ts">
import type { TunnelStatus } from '../api/gen';
import { formatBitRate } from '../format';
import { m } from '../paraglide/messages.js';
import { href } from '../router';
import type { TopologyModel } from '../topology';
import { internetChip, tailscaleChip } from '../topology-labels';
import GroupHeading from '../ui/GroupHeading.svelte';
import type { LoadStatus } from '../ui/loader.svelte';
import Section from '../ui/Section.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import Status from '../ui/Status.svelte';
import type { TunnelControl } from '../vpn-tunnel.svelte';
import VPNTunnelPanel from './VPNTunnelPanel.svelte';

// The things people check and toggle most: clients, live traffic, Tailscale, the VPN tunnel.
type Part = 'connectivity' | 'clients' | 'throughput' | 'tailscale';
let {
  model,
  status,
  vpn,
  tunnel,
}: {
  model: TopologyModel;
  status: Record<Part, LoadStatus>;
  vpn: { status: LoadStatus; data?: TunnelStatus; retry: () => void };
  tunnel: TunnelControl;
} = $props();

const active = $derived(model.sources.find((s) => s.state === 'active'));
// Stale per row: a known value whose source is reloading or failed stays, dimmed and marked.
const stale = (part: Part, known: boolean) => known && status[part].state !== 'ready';
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
      {@const chip = tailscaleChip[model.tailscale]}
      <SettingsRow label={m.tailscale_title()}>
        {#snippet control()}
          {@const old = stale('tailscale', true)}
          <a href={href('vpn-tailscale')} class={old ? 'opacity-60' : ''}><Status kind={chip.kind}>{chip.label()}</Status></a>
          {@render staleMark(old)}
        {/snippet}
      </SettingsRow>
    {/if}
  </div>
  <div class="flex flex-col gap-2 border-t border-border pt-4">
    <GroupHeading label={m.dashboard_vpn()} />
    <VPNTunnelPanel status={vpn} control={tunnel} canEnable={(snapshot) => snapshot.state === 'configured'} />
    <a href={href('vpn')} class="self-start text-sm text-muted-foreground underline-offset-4 hover:underline">{m.dashboard_vpn_settings()}</a>
  </div>
</Section>
