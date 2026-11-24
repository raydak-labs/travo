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

// One card per uplink, like the original Travo: what it is connected to, its address and live
// traffic. On phones these carry what the network map shows on desktop.
let {
  sources,
  uplinks,
}: {
  sources: TopologySource[];
  uplinks: { status: LoadStatus; data?: DashboardUplinks; retry: () => void };
} = $props();

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
</script>

<div class="grid items-start gap-4 sm:grid-cols-2 lg:grid-cols-3">
  {#each sources as source (source.name)}
    {@const Icon = icons[source.name]}
    {@const u = raw(source.name)}
    <Section title={sourceLabel[source.name]()}>
      {#snippet actions()}
        <Status kind={kind[source.state]}>{stateLabel[source.state]()}</Status>
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
            <dt class="text-muted-foreground">{m.uplink_card_address()}</dt>
            <dd class="text-right font-mono break-all">{u?.ipv4 ?? '–'}</dd>
            {#if u?.device}
              <dt class="text-muted-foreground">{m.uplink_card_device()}</dt>
              <dd class="text-right font-mono">{u.device}</dd>
            {/if}
            {#if source.rate}
              <dt class="text-muted-foreground">{m.uplink_card_traffic()}</dt>
              <dd class="text-right tabular-nums">↓ {formatBitRate(source.rate.rxBitsPerSecond)} · ↑ {formatBitRate(source.rate.txBitsPerSecond)}</dd>
            {/if}
          </dl>
        {/if}
      </LoadState>
    </Section>
  {/each}
</div>
