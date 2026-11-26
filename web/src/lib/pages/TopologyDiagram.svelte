<script lang="ts">
import Cable from '@lucide/svelte/icons/cable';
import Monitor from '@lucide/svelte/icons/monitor';
import RouterIcon from '@lucide/svelte/icons/router';
import Smartphone from '@lucide/svelte/icons/smartphone';
import Wifi from '@lucide/svelte/icons/wifi';
import * as HoverCard from '$lib/components/ui/hover-card';
import Skeleton from '$lib/components/ui/skeleton/skeleton.svelte';
import type { DashboardUplinks, SystemInfoBody } from '../api/gen';
import { formatBitRate, formatBytes, formatUptime } from '../format';
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
import MapBadge from '../ui/MapBadge.svelte';
import Status from '../ui/Status.svelte';
import type { StatusKind } from '../ui/status';
import { uplinkFacts } from '../uplink-facts';

// The dashboard map in the original Travo look across the full width: uplinks on the left
// (label, then icon), thin curved links into a square router tile, client counts on the right.
// Icons and links share one coordinate system, so every link ends in an icon centre. Hovering
// or focusing a node shows its details. Desktop only (docs/web-ui.md "Dashboard network map").
type Part = 'uplinks' | 'connectivity' | 'throughput' | 'clients';
let {
  model,
  status,
  uplinks,
  resources,
  ipv6,
}: {
  model: TopologyModel;
  status: Record<Part, LoadStatus> & { vpn?: LoadStatus };
  uplinks?: DashboardUplinks;
  resources?: SystemInfoBody;
  /** IPv6 on the LAN; undefined while unknown, which hides the badge. */
  ipv6?: boolean;
} = $props();

const captionId = $props.id();
const icons: Record<UplinkName, typeof Wifi> = { wan: Cable, wwan: Wifi, usbtether: Smartphone };
const configure: Record<UplinkName, string> = {
  wan: href('internet'),
  wwan: href('wifi'),
  usbtether: href('internet'),
};

// Geometry: x in percent of the width, y in px; the svg viewBox is 1000 × HEIGHT, stretched.
const HEIGHT = 240;
const X = { source: 27, router: 48, client: 69 };
const rowY = (i: number, count: number) => ((i + 0.5) / count) * HEIGHT;
const MID = HEIGHT / 2;
const curve = (x1: number, y1: number, x2: number, y2: number) => {
  const mid = ((x1 + x2) / 2) * 10;
  return `M ${x1 * 10} ${y1} C ${mid} ${y1}, ${mid} ${y2}, ${x2 * 10} ${y2}`;
};
const rawUplink = (name: UplinkName) => uplinks?.uplinks.find((u) => u.name === name);

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

const sourceKind: Record<SourceState, StatusKind> = {
  active: 'ok',
  standby: 'info',
  down: 'warn',
  not_configured: 'neutral',
  unknown: 'neutral',
};
const clientRows = $derived([
  {
    key: 'wifi',
    icon: Wifi,
    count: model.clients?.wifi,
    text: (count: number) => m.topology_clients_wifi({ count }),
  },
  {
    key: 'lan',
    icon: Monitor,
    count: model.clients?.lan,
    text: (count: number) => m.topology_clients_lan({ count }),
  },
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

{#snippet facts(rows: [string, string][])}
  <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1">
    {#each rows as [label, value] (label)}
      <dt class="text-muted-foreground">{label}</dt>
      <dd class="text-right break-words">{value}</dd>
    {/each}
  </dl>
{/snippet}

{#if firstLoad}
  <Skeleton class="h-60 w-full" />
{:else}
  <figure class="m-0" aria-labelledby={captionId}>
    <figcaption id={captionId} class="sr-only">{m.topology_label()}</figcaption>
    <ul class="sr-only" data-testid="topology-summary">
      {#each model.sources as source (source.name)}<li>{sourceSummary(source)}</li>{/each}
      <li>{routerSummary(model)}</li>
      <li>{clientsSummary(model.clients)}</li>
    </ul>

    <div class="relative" style="height: {HEIGHT}px">
      <svg aria-hidden="true" class="absolute inset-0 h-full w-full" viewBox="0 0 1000 {HEIGHT}" preserveAspectRatio="none">
        {#each model.sources as source, i (source.name)}
          {#if source.state !== 'not_configured'}
            <path
              d={curve(X.source, rowY(i, model.sources.length), X.router, MID)}
              class="topology-link topology-link-{source.state}"
              vector-effect="non-scaling-stroke"
            />
          {/if}
        {/each}
        {#each clientRows as row, i (row.key)}
          <path
            d={curve(X.router, MID, X.client, rowY(i, clientRows.length))}
            class="topology-link topology-link-{clientLink(row.count)}"
            vector-effect="non-scaling-stroke"
          />
        {/each}
      </svg>

      <ul data-part="sources">
        {#each model.sources as source, i (source.name)}
          {@const Icon = icons[source.name]}
          {@const muted = source.state === 'not_configured'}
          <li
            class="absolute flex items-center"
            style="top: {rowY(i, model.sources.length)}px; right: {100 - X.source}%; transform: translate(1.25rem, -50%); max-width: {X.source}%"
            data-state={source.state}
          >
            <HoverCard.Root openDelay={150} closeDelay={100}>
              <HoverCard.Trigger
                href={configure[source.name]}
                class="flex min-w-0 items-center gap-3 rounded-md text-right focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none {muted || sourcesStale ? 'opacity-60' : ''}"
              >
                <span class="min-w-0">
                  <span class="block truncate text-sm font-medium">{sourceLabel[source.name]()}</span>
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
                <span class="relative flex size-10 shrink-0 items-center justify-center rounded-lg border bg-card {source.state === 'active' ? 'topology-node-active' : 'border-border text-muted-foreground'}">
                  <Icon aria-hidden="true" size={18} />
                  <span class="absolute -top-1 -right-1 size-2.5 rounded-full ring-2 ring-card topology-dot-{source.state}" aria-hidden="true"></span>
                </span>
              </HoverCard.Trigger>
              <HoverCard.Content side="bottom" class="w-72" portalProps={{ disabled: typeof window === 'undefined' }}>
                <div class="mb-2 flex items-center justify-between gap-2">
                  <span class="font-semibold">{sourceLabel[source.name]()}</span>
                  <Status kind={sourceKind[source.state]}>{stateLabel[source.state]()}</Status>
                </div>
                {@render facts(uplinkFacts(source, rawUplink(source.name)))}
                <a href={configure[source.name]} class="mt-3 block text-right text-xs font-medium text-primary hover:underline">{m.topology_open_settings()}</a>
              </HoverCard.Content>
            </HoverCard.Root>
          </li>
        {/each}
      </ul>

      <div
        data-part="router"
        class="absolute flex w-56 flex-col items-center gap-1 text-center {routerStale ? 'opacity-60' : ''}"
        style="left: {X.router}%; top: {MID}px; transform: translate(-50%, -2.25rem)"
      >
        <HoverCard.Root openDelay={150} closeDelay={100}>
          <HoverCard.Trigger href={href('system')} class="relative flex size-18 items-center justify-center rounded-2xl border border-border bg-muted focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none" aria-label={model.router?.hostname ?? m.topology_router()}>
            <RouterIcon aria-hidden="true" size={36} class="text-foreground/80" />
            <span class="absolute -top-1 -right-1 size-3 rounded-full ring-2 ring-card topology-dot-{routerDot}" aria-hidden="true"></span>
          </HoverCard.Trigger>
          <HoverCard.Content side="bottom" class="w-72" portalProps={{ disabled: typeof window === 'undefined' }}>
            <div class="mb-2 font-semibold">{model.router?.hostname ?? m.topology_router()}</div>
            {#if resources}
              {@render facts([
                [m.system_model(), resources.model],
                [m.system_openwrt(), resources.openwrt],
                [m.system_uptime(), formatUptime(resources.uptime_seconds)],
                [m.system_memory(), m.system_used_of({ used: formatBytes(resources.memory.total_bytes - resources.memory.available_bytes), total: formatBytes(resources.memory.total_bytes) })],
              ])}
            {/if}
            <a href={href('system')} class="mt-3 block text-right text-xs font-medium text-primary hover:underline">{m.topology_open_settings()}</a>
          </HoverCard.Content>
        </HoverCard.Root>
        <div class="mt-1 min-w-0">
          <div class="text-sm font-semibold">{model.router?.hostname ?? m.topology_router()}</div>
          {#if model.router}<div class="text-xs break-words text-muted-foreground">{model.router.model}</div>{/if}
        </div>
        <div class="mt-1 flex flex-nowrap justify-center gap-1" data-part="router-badges">
          <MapBadge label={m.topology_internet()} state={internetChip[model.internet].label()} kind={internetChip[model.internet].kind} href={href('internet')} />
          <MapBadge label={m.dashboard_vpn()} state={vpnChip[model.vpn].label()} kind={vpnChip[model.vpn].kind} href={href('vpn')} />
          {#if model.tailscale !== 'absent'}
            {@const chip = tailscaleChip[model.tailscale]}
            <MapBadge label={m.tailscale_title()} state={chip.label()} kind={chip.kind} href={href('vpn-tailscale')} />
          {/if}
          {#if ipv6 !== undefined}
            <MapBadge label={m.topology_ipv6()} state={ipv6 ? m.topology_on() : m.topology_off()} kind={ipv6 ? 'ok' : 'neutral'} href={href('network-ipv6')} />
          {/if}
        </div>
        {@render staleMark(routerStale)}
      </div>

      <div data-part="clients" class={clientsStale ? 'opacity-60' : ''}>
        {#each clientRows as row, i (row.key)}
          {@const Icon = row.icon}
          <a
            href={href('clients')}
            class="absolute flex items-center gap-3 rounded-md focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
            style="top: {rowY(i, clientRows.length)}px; left: {X.client}%; transform: translate(-1.25rem, -50%)"
          >
            <span class="flex size-10 items-center justify-center rounded-lg border bg-card {row.count ? 'topology-node-active' : 'border-border text-muted-foreground'}"><Icon aria-hidden="true" size={18} /></span>
            <span>
              <span class="block text-sm font-medium tabular-nums">{row.count === undefined ? '–' : row.text(row.count)}</span>
              {#if row.count === undefined}<span class="block text-xs text-muted-foreground">{m.topology_state_unknown()}</span>{/if}
            </span>
          </a>
        {/each}
        {#if model.clients?.partial || clientsStale}
          <div class="absolute bottom-0 flex flex-wrap gap-1" style="left: {X.client}%">
            {#if model.clients?.partial}<Status kind="warn">{m.topology_clients_incomplete()}</Status>{/if}
            {@render staleMark(clientsStale)}
          </div>
        {/if}
      </div>
    </div>
  </figure>
{/if}
