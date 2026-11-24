<script lang="ts">
import type { TunnelStatus } from '../api/gen';
import { formatBitRate } from '../format';
import { m } from '../paraglide/messages.js';
import { href } from '../router';
import type { TopologyModel } from '../topology';
import { tailscaleChip } from '../topology-labels';
import GroupHeading from '../ui/GroupHeading.svelte';
import type { LoadStatus } from '../ui/loader.svelte';
import Section from '../ui/Section.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import Status from '../ui/Status.svelte';
import type { TunnelControl } from '../vpn-tunnel.svelte';
import VPNTunnelPanel from './VPNTunnelPanel.svelte';

// The things people check and toggle most: clients, live traffic, Tailscale, the VPN tunnel.
let {
  model,
  vpn,
  tunnel,
}: {
  model: TopologyModel;
  vpn: { status: LoadStatus; data?: TunnelStatus; retry: () => void };
  tunnel: TunnelControl;
} = $props();

const active = $derived(model.sources.find((s) => s.state === 'active'));
</script>

<Section title={m.dashboard_quick_status()}>
  <div>
    <SettingsRow label={m.nav_clients()}>
      {#snippet control()}
        <a href={href('clients')} class="tabular-nums underline-offset-4 hover:underline">
          {model.clients
            ? `${m.topology_clients_wifi({ count: model.clients.wifi })} · ${m.topology_clients_lan({ count: model.clients.lan })}`
            : m.topology_state_unknown()}
        </a>
      {/snippet}
    </SettingsRow>
    <SettingsRow label={m.uplink_card_traffic()}>
      {#snippet control()}
        <span class="text-sm tabular-nums">
          {#if active?.rate}
            ↓ {formatBitRate(active.rate.rxBitsPerSecond)} · ↑ {formatBitRate(active.rate.txBitsPerSecond)}
          {:else}
            {m.dashboard_rate_unknown()}
          {/if}
        </span>
      {/snippet}
    </SettingsRow>
    {#if model.tailscale !== 'absent'}
      {@const chip = tailscaleChip[model.tailscale]}
      <SettingsRow label={m.tailscale_title()}>
        {#snippet control()}
          <a href={href('vpn-tailscale')}><Status kind={chip.kind}>{chip.label()}</Status></a>
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
