<script lang="ts">
import Cable from '@lucide/svelte/icons/cable';
import Laptop from '@lucide/svelte/icons/laptop';
import RouterIcon from '@lucide/svelte/icons/router';
import Usb from '@lucide/svelte/icons/usb';
import Wifi from '@lucide/svelte/icons/wifi';
import Skeleton from '$lib/components/ui/skeleton/skeleton.svelte';
import { formatBitRate } from '../format';
import { m } from '../paraglide/messages.js';
import { href } from '../router';
import type { SourceState, TopologyModel, UplinkName } from '../topology';
import {
  clientsSummary,
  internetChip,
  routerSummary,
  sourceLabel,
  sourceSummary,
  stateLabel,
  tailscaleChip,
  vpnChip,
} from '../topology-labels';
import type { LoadStatus } from '../ui/loader.svelte';
import Status from '../ui/Status.svelte';

type Part = 'uplinks' | 'connectivity' | 'throughput' | 'clients';
let {
  model,
  status,
}: { model: TopologyModel; status: Record<Part, LoadStatus> & { vpn?: LoadStatus } } = $props();

const captionId = $props.id();
const icons: Record<UplinkName, typeof Wifi> = { wan: Cable, wwan: Wifi, usbtether: Usb };
const configure: Record<UplinkName, string> = {
  wan: href('network-wan'),
  wwan: href('wifi'),
  usbtether: href('internet'),
};

// Stale: the part has a known value but its source is reloading or failed (docs/web-ui.md).
const refreshing = (s: LoadStatus) => s.state !== 'ready';
const sourcesKnown = $derived(model.sources.some((s) => s.state !== 'unknown'));
// States come from uplinks and connectivity; a throughput failure only makes the rates stale.
const sourcesStale = $derived(
  sourcesKnown && [status.uplinks, status.connectivity].some(refreshing),
);
const ratesStale = $derived(refreshing(status.throughput));
const routerStale = $derived(
  (model.internet !== 'unknown' && refreshing(status.connectivity)) ||
    (model.vpn !== 'unknown' && status.vpn !== undefined && refreshing(status.vpn)),
);
const clientsStale = $derived(model.clients !== null && refreshing(status.clients));
const firstLoad = $derived(
  !sourcesKnown &&
    model.clients === null &&
    status.uplinks.state === 'loading' &&
    status.clients.state === 'loading',
);

// The stacked layout draws one connector for all sources: the best state among them.
const RANK: SourceState[] = ['active', 'standby', 'down', 'unknown', 'not_configured'];
const bestState = $derived(
  RANK.find((state) => model.sources.some((s) => s.state === state)) ?? 'unknown',
);
const clientCount = $derived(model.clients ? model.clients.wifi + model.clients.lan : 0);
const clientLink = $derived<SourceState>(
  model.clients === null ? 'unknown' : clientCount > 0 ? 'active' : 'down',
);
</script>

{#snippet staleMark(stale: boolean)}
  {#if stale}<Status kind="stale">{m.load_stale()}</Status>{/if}
{/snippet}

{#snippet stackLink(state: SourceState)}
  <svg aria-hidden="true" class="mx-auto h-6 w-2 @xl:hidden" viewBox="0 0 2 24" preserveAspectRatio="none">
    {#if state !== 'not_configured'}
      <line x1="1" y1="0" x2="1" y2="24" class="topology-link topology-link-{state}" vector-effect="non-scaling-stroke" />
    {/if}
  </svg>
{/snippet}

{#if firstLoad}
  <Skeleton class="h-48 w-full" />
{:else}
  <figure class="@container m-0" aria-labelledby={captionId}>
    <figcaption id={captionId} class="sr-only">{m.topology_label()}</figcaption>
    <ul class="sr-only" data-testid="topology-summary">
      {#each model.sources as source (source.name)}<li>{sourceSummary(source)}</li>{/each}
      <li>{routerSummary(model)}</li>
      <li>{clientsSummary(model.clients)}</li>
    </ul>

    <div class="flex flex-col @xl:grid @xl:grid-cols-[minmax(0,1fr)_2.5rem_minmax(0,1fr)_2.5rem_minmax(0,1fr)]">
      <ul data-part="sources" class="grid auto-rows-fr">
        {#each model.sources as source (source.name)}
          {@const Icon = icons[source.name]}
          <li class="py-1" data-state={source.state}>
            <a
              href={configure[source.name]}
              class="flex h-full flex-col gap-1 rounded-lg border border-border bg-card p-3 hover:bg-muted {source.state === 'not_configured' || sourcesStale ? 'opacity-60' : ''}"
            >
              <span class="flex items-center gap-2 font-medium">
                <Icon aria-hidden="true" size={16} class="shrink-0" />{sourceLabel[source.name]()}
              </span>
              <span class="text-xs">{stateLabel[source.state]()}</span>
              {#if source.detail}<span class="text-xs break-all text-muted-foreground">{source.detail}</span>{/if}
              {#if source.rate}
                <span class="text-xs tabular-nums text-muted-foreground {ratesStale && !sourcesStale ? 'opacity-60' : ''}" data-rate>
                  ↓ {formatBitRate(source.rate.rxBitsPerSecond)} · ↑ {formatBitRate(source.rate.txBitsPerSecond)}
                </span>
              {/if}
              {@render staleMark(sourcesStale)}
            </a>
          </li>
        {/each}
      </ul>

      <!-- Wide layout: one line from each source row to the router's middle. -->
      <svg aria-hidden="true" class="hidden h-full w-full @xl:block" viewBox="0 0 100 300" preserveAspectRatio="none">
        {#each model.sources as source, i (source.name)}
          {#if source.state !== 'not_configured'}
            <line
              x1="0"
              y1={50 + 100 * i}
              x2="100"
              y2="150"
              class="topology-link topology-link-{source.state}"
              vector-effect="non-scaling-stroke"
            />
          {/if}
        {/each}
      </svg>
      {@render stackLink(bestState)}

      <div data-part="router" class="flex flex-col gap-2 self-center rounded-lg border border-border bg-card p-3 {routerStale ? 'opacity-60' : ''}">
        <span class="flex items-center gap-2 font-medium">
          <RouterIcon aria-hidden="true" size={16} class="shrink-0" />{model.router?.hostname ?? m.topology_router()}
        </span>
        {#if model.router}<span class="text-xs text-muted-foreground">{model.router.model}</span>{/if}
        <dl class="grid grid-cols-[auto_1fr] items-center gap-x-2 gap-y-1 text-xs">
          <dt class="text-muted-foreground">{m.topology_internet()}</dt>
          <dd><Status kind={internetChip[model.internet].kind}>{internetChip[model.internet].label()}</Status></dd>
          <dt class="text-muted-foreground">{m.dashboard_vpn()}</dt>
          <dd><Status kind={vpnChip[model.vpn].kind}>{vpnChip[model.vpn].label()}</Status></dd>
          {#if model.tailscale !== 'absent'}
            {@const chip = tailscaleChip[model.tailscale]}
            <dt class="text-muted-foreground"><a href={href('vpn-tailscale')} class="underline-offset-2 hover:underline">{m.tailscale_title()}</a></dt>
            <dd><Status kind={chip.kind}>{chip.label()}</Status></dd>
          {/if}
        </dl>
        {@render staleMark(routerStale)}
      </div>

      <svg aria-hidden="true" class="hidden h-full w-full @xl:block" viewBox="0 0 100 300" preserveAspectRatio="none">
        <line x1="0" y1="150" x2="100" y2="150" class="topology-link topology-link-{clientLink}" vector-effect="non-scaling-stroke" />
      </svg>
      {@render stackLink(clientLink)}

      <div data-part="clients" class="flex flex-col gap-1 self-center rounded-lg border border-border bg-card p-3 {clientsStale ? 'opacity-60' : ''}">
        <span class="flex items-center gap-2 font-medium">
          <Laptop aria-hidden="true" size={16} class="shrink-0" />{m.nav_clients()}
        </span>
        {#if model.clients}
          <a href={href('clients')} class="text-xs underline-offset-2 hover:underline">{m.topology_clients_wifi({ count: model.clients.wifi })}</a>
          <a href={href('clients')} class="text-xs underline-offset-2 hover:underline">{m.topology_clients_lan({ count: model.clients.lan })}</a>
          {#if model.clients.partial}<Status kind="warn">{m.topology_clients_incomplete()}</Status>{/if}
        {:else}
          <span class="text-xs">{m.topology_state_unknown()}</span>
        {/if}
        {@render staleMark(clientsStale)}
      </div>
    </div>
  </figure>
{/if}
