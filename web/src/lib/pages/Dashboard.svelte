<script lang="ts">
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import * as Collapsible from '$lib/components/ui/collapsible';
import type { SystemInfoBody } from '../api/gen';
import { fetchHealth } from '../api/health';
import { fetchVpnStatus } from '../api/vpn-status';
import { captiveCardVisible } from '../captive';
import type { dashboardEvents } from '../dashboard-events.svelte';
import { formatBytes, formatLoad, formatUptime } from '../format';
import { m } from '../paraglide/messages.js';
import { href } from '../router';
import { topologyModel } from '../topology';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import PageHeader from '../ui/PageHeader.svelte';
import Section from '../ui/Section.svelte';
import Status from '../ui/Status.svelte';
import SummaryBand from '../ui/SummaryBand.svelte';
import SummaryTile from '../ui/SummaryTile.svelte';
import { createTunnelControl } from '../vpn-tunnel.svelte';
import CaptivePortalCard from './CaptivePortalCard.svelte';
import InterfaceTraffic from './InterfaceTraffic.svelte';
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';
import TopologyDiagram from './TopologyDiagram.svelte';
import VPNTunnelPanel from './VPNTunnelPanel.svelte';

// Own chunk: the history card loads after the live cards (#232).
const historyCard = lazyPage(() => import('./StatsHistoryCard.svelte'));

let {
  events,
}: {
  events: Pick<
    typeof dashboardEvents,
    'connectivity' | 'resources' | 'throughput' | 'tailscale' | 'uplinks' | 'clients'
  >;
} = $props();

const connectivity = $derived(events.connectivity);
const health = createLoader(fetchHealth);
const resources = $derived(events.resources);
const throughput = $derived(events.throughput);
const tailscale = $derived(events.tailscale);
// Same status poll and guarded toggle as the VPN page: Enable, then Confirm (two taps).
const vpn = createLoader(fetchVpnStatus, 10000);
const tunnel = createTunnelControl(() => vpn.retry());
const topology = $derived(
  topologyModel({
    uplinks: events.uplinks.data,
    connectivity: connectivity.data,
    throughput: throughput.data,
    clients: events.clients.data,
    resources: resources.data,
    tailscale: tailscale.data,
    vpn: vpn.data,
  }),
);
onDestroy(() => {
  vpn.stop();
  tunnel.stop();
});

let trafficOpen = $state(false);
</script>

{#snippet resource(label: string, value: (info: SystemInfoBody) => string)}
  <SummaryTile {label}>
    <LoadState status={resources.status} retry={resources.retry} stale={resources.data !== undefined}>
      {#if resources.data}<span class="break-all tabular-nums">{value(resources.data)}</span>{/if}
    </LoadState>
  </SummaryTile>
{/snippet}


<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_dashboard()}>
    {#snippet actions()}
      <Button size="sm" href={href('wifi')}>{m.dashboard_connect_to_wifi()}</Button>
    {/snippet}
  </PageHeader>

  {#if connectivity.data && captiveCardVisible(connectivity.data)}
    <CaptivePortalCard info={connectivity.data} refresh={connectivity.retry} />
  {/if}

  <Section title={m.topology_title()}>
    <TopologyDiagram
      model={topology}
      status={{
        uplinks: events.uplinks.status,
        connectivity: connectivity.status,
        throughput: throughput.status,
        clients: events.clients.status,
        vpn: vpn.status,
      }}
    />
  </Section>

  <SummaryBand title={m.dashboard_resources()}>
    {@render resource(m.dashboard_cpu_load(), (info) => formatLoad([info.load[0]]))}
    {@render resource(m.system_memory(), (info) =>
      info.memory.total_bytes === 0
        ? m.dashboard_unavailable()
        : m.system_used_of({
            used: formatBytes(Math.max(0, info.memory.total_bytes - info.memory.available_bytes)),
            total: formatBytes(info.memory.total_bytes),
          }),
    )}
    {@render resource(m.system_uptime(), (info) => formatUptime(info.uptime_seconds))}
    {@render resource(m.dashboard_version(), (info) => info.app.version)}
    <SummaryTile label={m.dashboard_daemon()}>
      <LoadState status={health.status} retry={health.retry}>
        <div><Status kind="ok">{m.status_ok()}</Status></div>
      </LoadState>
    </SummaryTile>
  </SummaryBand>

  <Section title={m.dashboard_vpn()}>
    <VPNTunnelPanel status={vpn} control={tunnel} canEnable={(snapshot) => snapshot.state === 'configured'} />
    <div class="flex flex-wrap gap-x-4">
      <Button variant="link" size="sm" class="px-0" href={href('vpn')}>{m.dashboard_vpn_settings()}</Button>
      {#if tailscale.data?.installed}
        <Button variant="link" size="sm" class="px-0" href={href('vpn-tailscale')}>{m.tailscale_open()}</Button>
      {/if}
    </div>
  </Section>

  <Section title={m.dashboard_throughput()} description={m.dashboard_traffic_description()}>
    <Collapsible.Root bind:open={trafficOpen}>
      <Collapsible.Trigger>
        {#snippet child({ props })}
          <Button {...props} variant="outline" size="sm" type="button">
            {trafficOpen ? m.dashboard_traffic_hide() : m.dashboard_traffic_show()}
          </Button>
        {/snippet}
      </Collapsible.Trigger>
      <Collapsible.Content class="mt-4">
        <InterfaceTraffic {throughput} />
      </Collapsible.Content>
    </Collapsible.Root>
  </Section>

  <PageHost load={historyCard} />
</div>
