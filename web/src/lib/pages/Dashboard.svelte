<script lang="ts">
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import * as Collapsible from '$lib/components/ui/collapsible';
import { fetchVpnStatus } from '../api/vpn-status';
import { captiveCardVisible } from '../captive';
import type { dashboardEvents } from '../dashboard-events.svelte';
import { m } from '../paraglide/messages.js';
import { href } from '../router';
import { topologyModel } from '../topology';
import { createLoader } from '../ui/loader.svelte';
import PageHeader from '../ui/PageHeader.svelte';
import Section from '../ui/Section.svelte';
import { createTunnelControl } from '../vpn-tunnel.svelte';
import CaptivePortalCard from './CaptivePortalCard.svelte';
import InterfaceTraffic from './InterfaceTraffic.svelte';
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';
import QuickStatus from './QuickStatus.svelte';
import SystemCard from './SystemCard.svelte';
import TopologyDiagram from './TopologyDiagram.svelte';
import UplinkCards from './UplinkCards.svelte';

// Own chunk: the history card loads after the live cards (#232).
const historyCard = lazyPage(() => import('./StatsHistoryCard.svelte'));

let {
  events,
}: {
  events: Pick<
    typeof dashboardEvents,
    'connectivity' | 'resources' | 'throughput' | 'tailscale' | 'uplinks' | 'clients' | 'alerts'
  >;
} = $props();

const connectivity = $derived(events.connectivity);
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

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_dashboard()} description={m.dashboard_description()}>
    {#snippet actions()}
      <Button size="sm" href={href('wifi')}>{m.dashboard_connect_to_wifi()}</Button>
    {/snippet}
  </PageHeader>

  {#if connectivity.data && captiveCardVisible(connectivity.data)}
    <CaptivePortalCard info={connectivity.data} refresh={connectivity.retry} />
  {/if}

  <!-- The map needs width; phones and narrow windows get the same facts from the cards below. -->
  <div class="hidden lg:block">
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
  </div>

  <UplinkCards sources={topology.sources} uplinks={events.uplinks} />

  <div class="grid items-start gap-4 lg:grid-cols-2">
    <QuickStatus model={topology} {vpn} {tunnel} />
    <SystemCard {resources} alerts={events.alerts.data?.alerts ?? []} />
  </div>

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
