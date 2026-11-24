<script lang="ts">
import Cable from '@lucide/svelte/icons/cable';
import Monitor from '@lucide/svelte/icons/monitor';
import RouterIcon from '@lucide/svelte/icons/router';
import Smartphone from '@lucide/svelte/icons/smartphone';
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

// The dashboard map in the original Travo look: uplinks as light rows on the left, thin curved
// links into a square router tile, client counts on the right. Desktop only; phones get the
// dashboard cards instead (docs/web-ui.md "Dashboard network map").
type Part = 'uplinks' | 'connectivity' | 'throughput' | 'clients';
let {
  model,
  status,
}: { model: TopologyModel; status: Record<Part, LoadStatus> & { vpn?: LoadStatus } } = $props();

const captionId = $props.id();
const icons: Record<UplinkName, typeof Wifi> = { wan: Cable, wwan: Wifi, usbtether: Smartphone };
const configure: Record<UplinkName, string> = {
  wan: href('network-wan'),
  wwan: href('wifi'),
  usbtether: href('internet'),
};

// Geometry: rows are ROW px apart; lines are drawn in a 100 × HEIGHT box stretched to the gap.
const ROW = 56;
const HEIGHT = 4 * ROW;
const sourceY = (i: number, count: number) => ((i + 0.5) / count) * HEIGHT;
const MID = HEIGHT / 2;
const curve = (y1: number, y2: number) => `M 0 ${y1} C 50 ${y1}, 50 ${y2}, 100 ${y2}`;

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

const clientRows = $derived([
  { key: 'wifi', icon: Wifi, count: model.clients?.wifi, label: m.topology_clients_wifi_label() },
  { key: 'lan', icon: Monitor, count: model.clients?.lan, label: m.topology_clients_lan_label() },
]);
const clientLink = (count: number | undefined): SourceState =>
  count === undefined ? 'unknown' : count > 0 ? 'active' : 'down';
const routerDot = $derived(
  { online: 'active', captive: 'warn', offline: 'danger', unknown: 'down' }[model.internet],
);
</script>

{#snippet staleMark(stale: boolean)}
  {#if stale}<Status kind="stale">{m.load_stale()}</Status>{/if}
{/snippet}

{#if firstLoad}
  <Skeleton class="h-56 w-full" />
{:else}
  <figure class="m-0" aria-labelledby={captionId}>
    <figcaption id={captionId} class="sr-only">{m.topology_label()}</figcaption>
    <ul class="sr-only" data-testid="topology-summary">
      {#each model.sources as source (source.name)}<li>{sourceSummary(source)}</li>{/each}
      <li>{routerSummary(model)}</li>
      <li>{clientsSummary(model.clients)}</li>
    </ul>

    <div class="flex items-stretch" style="height: {HEIGHT}px">
      <ul data-part="sources" class="relative w-48 shrink-0">
        {#each model.sources as source, i (source.name)}
          {@const Icon = icons[source.name]}
          <li
            class="absolute inset-x-0 -translate-y-1/2"
            style="top: {sourceY(i, model.sources.length)}px"
            data-state={source.state}
          >
            <a
              href={configure[source.name]}
              class="flex items-start gap-2 rounded-md px-1 py-0.5 hover:bg-muted {source.state === 'not_configured' || sourcesStale ? 'opacity-60' : ''}"
            >
              <span class="mt-1.5 size-2 shrink-0 rounded-full topology-dot-{source.state}" aria-hidden="true"></span>
              <Icon aria-hidden="true" size={16} class="mt-0.5 shrink-0 {source.state === 'active' ? 'topology-icon-active' : 'text-muted-foreground'}" />
              <span class="min-w-0">
                <span class="block text-xs font-medium">{sourceLabel[source.name]()}</span>
                <span class="block truncate text-xs text-muted-foreground">
                  {stateLabel[source.state]()}{#if source.detail}{' · '}{source.detail}{/if}
                </span>
                {#if source.rate}
                  <span class="block text-xs tabular-nums text-muted-foreground {ratesStale && !sourcesStale ? 'opacity-60' : ''}" data-rate>
                    ↓ {formatBitRate(source.rate.rxBitsPerSecond)} · ↑ {formatBitRate(source.rate.txBitsPerSecond)}
                  </span>
                {/if}
                {@render staleMark(sourcesStale)}
              </span>
            </a>
          </li>
        {/each}
      </ul>

      <svg aria-hidden="true" class="min-w-10 flex-1" viewBox="0 0 100 {HEIGHT}" preserveAspectRatio="none">
        {#each model.sources as source, i (source.name)}
          {#if source.state !== 'not_configured'}
            <path
              d={curve(sourceY(i, model.sources.length), MID)}
              class="topology-link topology-link-{source.state}"
              vector-effect="non-scaling-stroke"
            />
          {/if}
        {/each}
      </svg>

      <div data-part="router" class="flex w-44 shrink-0 flex-col items-center justify-center gap-2 text-center {routerStale ? 'opacity-60' : ''}">
        <div class="relative flex size-20 items-center justify-center rounded-2xl border border-border bg-muted">
          <RouterIcon aria-hidden="true" size={40} class="text-foreground/80" />
          <span class="absolute -top-1 -right-1 size-3 rounded-full ring-2 ring-card topology-dot-{routerDot}" aria-hidden="true"></span>
        </div>
        <div class="min-w-0">
          <div class="text-xs font-semibold">{model.router?.hostname ?? m.topology_router()}</div>
          {#if model.router}<div class="text-xs break-words text-muted-foreground">{model.router.model}</div>{/if}
        </div>
        <div class="flex flex-wrap justify-center gap-1">
          <Status kind={internetChip[model.internet].kind}>{m.topology_internet()} {internetChip[model.internet].label()}</Status>
          <Status kind={vpnChip[model.vpn].kind}>{m.dashboard_vpn()} {vpnChip[model.vpn].label()}</Status>
          {#if model.tailscale !== 'absent'}
            {@const chip = tailscaleChip[model.tailscale]}
            <a href={href('vpn-tailscale')}><Status kind={chip.kind}>{m.tailscale_title()} {chip.label()}</Status></a>
          {/if}
        </div>
        {@render staleMark(routerStale)}
      </div>

      <svg aria-hidden="true" class="min-w-10 flex-1" viewBox="0 0 100 {HEIGHT}" preserveAspectRatio="none">
        {#each clientRows as row, i (row.key)}
          <path
            d={curve(MID, sourceY(i, clientRows.length))}
            class="topology-link topology-link-{clientLink(row.count)}"
            vector-effect="non-scaling-stroke"
          />
        {/each}
      </svg>

      <div data-part="clients" class="relative w-40 shrink-0 {clientsStale ? 'opacity-60' : ''}">
        {#each clientRows as row, i (row.key)}
          {@const Icon = row.icon}
          <a
            href={href('clients')}
            class="absolute inset-x-0 flex -translate-y-1/2 items-center gap-3 rounded-md px-1 hover:bg-muted"
            style="top: {sourceY(i, clientRows.length)}px"
          >
            <span class="rounded-lg border border-border bg-muted p-1.5"><Icon aria-hidden="true" size={16} class="text-muted-foreground" /></span>
            <span>
              <span class="block text-2xl leading-none font-bold tabular-nums">{row.count ?? '–'}</span>
              <span class="block text-xs text-muted-foreground">{row.count === undefined ? m.topology_state_unknown() : row.label}</span>
            </span>
          </a>
        {/each}
        {#if model.clients?.partial || clientsStale}
          <div class="absolute inset-x-0 bottom-0 flex flex-wrap gap-1">
            {#if model.clients?.partial}<Status kind="warn">{m.topology_clients_incomplete()}</Status>{/if}
            {@render staleMark(clientsStale)}
          </div>
        {/if}
      </div>
    </div>
  </figure>
{/if}
