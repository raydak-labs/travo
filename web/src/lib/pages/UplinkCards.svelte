<script lang="ts">
import Cable from '@lucide/svelte/icons/cable';
import Smartphone from '@lucide/svelte/icons/smartphone';
import Wifi from '@lucide/svelte/icons/wifi';
import Button from '$lib/components/ui/button/button.svelte';
import type { DashboardUplinks } from '../api/gen';
import { formatBitRate } from '../format';
import { m } from '../paraglide/messages.js';
import { href } from '../router';
import type { SourceState, TopologySource, UplinkName } from '../topology';
import { sourceLabel, stateLabel } from '../topology-labels';
import LoadState from '../ui/LoadState.svelte';
import type { LoadStatus } from '../ui/loader.svelte';
import Section from '../ui/Section.svelte';
import Status from '../ui/Status.svelte';
import type { StatusKind } from '../ui/status';
import { bandLabel } from '../wifi';

// One card per uplink, like the original Travo: what it is connected to, its address and live
// traffic. On phones these carry what the network map shows on desktop.
let {
  sources,
  uplinks,
  status,
}: {
  sources: TopologySource[];
  uplinks: { status: LoadStatus; data?: DashboardUplinks; retry: () => void };
  /** The state also comes from connectivity, the rate from throughput: each marks its own part stale. */
  status: { connectivity: LoadStatus; throughput: LoadStatus };
} = $props();

const stateStale = $derived(status.connectivity.state !== 'ready');
const rateStale = $derived(status.throughput.state !== 'ready');

const icons: Record<UplinkName, typeof Wifi> = { wan: Cable, wwan: Wifi, usbtether: Smartphone };
const configure: Record<UplinkName, string> = {
  wan: href('network-wan'),
  wwan: href('wifi'),
  usbtether: href('internet'),
};
const kind: Record<SourceState, StatusKind> = {
  active: 'ok',
  standby: 'info',
  down: 'warn',
  not_configured: 'neutral',
  unknown: 'neutral',
};
const detailLabel: Record<UplinkName, () => string> = {
  wan: m.uplink_card_address,
  wwan: m.uplink_card_network,
  usbtether: m.uplink_card_link,
};
const raw = (name: UplinkName) => uplinks.data?.uplinks.find((u) => u.name === name);
const protoLabels: Record<string, () => string> = {
  dhcp: m.wan_proto_dhcp,
  static: m.wan_proto_static,
  pppoe: m.wan_proto_pppoe,
};
const protoLabel = (proto: string) => protoLabels[proto]?.() ?? proto;
</script>

<div class="grid items-start gap-4 sm:grid-cols-2 lg:grid-cols-3">
  {#each sources as source (source.name)}
    {@const Icon = icons[source.name]}
    {@const u = raw(source.name)}
    <Section title={sourceLabel[source.name]()}>
      {#snippet actions()}
        <span class={stateStale && source.state !== 'unknown' ? 'opacity-60' : ''}>
          <Status kind={kind[source.state]}>{stateLabel[source.state]()}</Status>
        </span>
        {#if stateStale && source.state !== 'unknown'}<Status kind="stale">{m.load_stale()}</Status>{/if}
      {/snippet}
      <LoadState status={uplinks.status} retry={uplinks.retry} stale={uplinks.data !== undefined}>
        {#if source.state === 'not_configured'}
          <div class="flex items-center gap-3 text-muted-foreground">
            <Icon aria-hidden="true" size={20} />
            <Button variant="outline" size="sm" href={configure[source.name]}>{m.uplink_card_setup()}</Button>
          </div>
        {:else}
          <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2">
            {#if source.name !== 'wan' && source.detail}
              <dt class="text-muted-foreground">{detailLabel[source.name]()}</dt>
              <dd class="text-right break-all">{source.detail}</dd>
            {/if}
            {#if source.name === 'wan' && u?.proto}
              <dt class="text-muted-foreground">{m.uplink_card_protocol()}</dt>
              <dd class="text-right">{protoLabel(u.proto)}</dd>
            {/if}
            {#if source.name === 'wwan' && u?.band}
              <dt class="text-muted-foreground">{m.uplink_card_band()}</dt>
              <dd class="text-right">{bandLabel(u.band)}</dd>
            {/if}
            {#if source.name === 'wwan' && u?.signal_dbm != null}
              <dt class="text-muted-foreground">{m.uplink_card_signal()}</dt>
              <dd class="text-right tabular-nums">{m.clients_signal({ signal: u.signal_dbm })}</dd>
            {/if}
            <dt class="text-muted-foreground">{m.uplink_card_address()}</dt>
            <dd class="text-right font-mono whitespace-nowrap">{u?.ipv4 ?? '–'}</dd>
            {#if u?.gateway}
              <dt class="text-muted-foreground">{m.uplink_card_gateway()}</dt>
              <dd class="text-right font-mono whitespace-nowrap">{u.gateway}</dd>
            {/if}
            {#if u?.device}
              <dt class="text-muted-foreground">{m.uplink_card_device()}</dt>
              <dd class="text-right font-mono">{u.device}</dd>
            {/if}
            {#if source.rate}
              <dt class="text-muted-foreground">{m.uplink_card_traffic()}</dt>
              <dd class="text-right tabular-nums {rateStale ? 'opacity-60' : ''}" data-rate>
                ↓ {formatBitRate(source.rate.rxBitsPerSecond)} · ↑ {formatBitRate(source.rate.txBitsPerSecond)}
                {#if rateStale}<Status kind="stale">{m.load_stale()}</Status>{/if}
              </dd>
            {/if}
          </dl>
        {/if}
      </LoadState>
    </Section>
  {/each}
</div>
