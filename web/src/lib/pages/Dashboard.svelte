<script lang="ts">
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import * as Table from '$lib/components/ui/table';
import { fetchHealth } from '../api/health';
import { fetchVpnStatus } from '../api/vpn-status';
import { captiveCardVisible } from '../captive';
import type { dashboardEvents } from '../dashboard-events.svelte';
import { formatBytes, formatLoad, formatUptime } from '../format';
import { m } from '../paraglide/messages.js';
import { href } from '../router';
import Card from '../ui/Card.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import Status from '../ui/Status.svelte';
import { createTunnelControl } from '../vpn-tunnel.svelte';
import CaptivePortalCard from './CaptivePortalCard.svelte';
import VPNTunnelPanel from './VPNTunnelPanel.svelte';

let {
  events,
}: { events: Pick<typeof dashboardEvents, 'connectivity' | 'resources' | 'throughput'> } = $props();

const connectivity = $derived(events.connectivity);
const health = createLoader(fetchHealth);
const resources = $derived(events.resources);
const throughput = $derived(events.throughput);
// Same status poll and guarded toggle as the VPN page: Enable, then Confirm (two taps).
const vpn = createLoader(fetchVpnStatus, 10000);
const tunnel = createTunnelControl(() => vpn.retry());
onDestroy(() => {
  vpn.stop();
  tunnel.stop();
});

const internetStatus = {
  online: { kind: 'ok', label: m.dashboard_internet_online },
  offline: { kind: 'danger', label: m.dashboard_internet_offline },
  captive: { kind: 'warn', label: m.dashboard_internet_captive },
  unknown: { kind: 'neutral', label: m.dashboard_connectivity_unknown },
} as const;

const mediumLabel = {
  ethernet: m.dashboard_medium_ethernet,
  wifi: m.dashboard_medium_wifi,
  usb: m.dashboard_medium_usb,
  virtual: m.dashboard_medium_virtual,
} as const;

function formatRate(rate: number | null): string {
  if (rate === null) return m.dashboard_rate_unknown();
  const rounded = rate < 1024 ? Math.round(rate * 10) / 10 : rate;
  return m.dashboard_bytes_per_second({ bytes: formatBytes(rounded) });
}
</script>

<div class="flex flex-col gap-4">
  {#if connectivity.data && captiveCardVisible(connectivity.data)}
    <CaptivePortalCard info={connectivity.data} refresh={connectivity.retry} />
  {/if}
  <Card title={m.dashboard_connectivity()}>
    <LoadState status={connectivity.status} retry={connectivity.retry} stale={connectivity.data !== undefined}>
      {#if connectivity.data}
        {@const info = connectivity.data}
        {@const internet = internetStatus[info.internet]}
        <dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2">
          <dt class="text-muted-foreground">{m.dashboard_active_uplink()}</dt>
          <dd class="break-all">
            {info.active_interface ?? m.dashboard_connectivity_unknown()}
            {#if info.medium}<span class="text-muted-foreground"> · {mediumLabel[info.medium]()}</span>{/if}
          </dd>
          <dt class="text-muted-foreground">{m.dashboard_carrier()}</dt>
          <dd>
            {#if info.carrier === null}
              <Status kind="neutral">{m.dashboard_connectivity_unknown()}</Status>
            {:else if info.carrier}
              <Status kind="ok">{m.dashboard_carrier_up()}</Status>
            {:else}
              <Status kind="danger">{m.dashboard_carrier_down()}</Status>
            {/if}
          </dd>
          <dt class="text-muted-foreground">{m.dashboard_internet()}</dt>
          <dd><Status kind={internet.kind}>{internet.label()}</Status></dd>
        </dl>
        {#if connectivity.status.state === 'ready'}
          <Button variant="outline" size="sm"
            type="button"
            class="w-fit text-sm"
            onclick={connectivity.retry}
          >
            {m.dashboard_refresh()}
          </Button>
        {/if}
      {/if}
    </LoadState>
    <Button size="sm" class="w-fit" href={href('wifi')}>
      {m.dashboard_connect_to_wifi()}
    </Button>
  </Card>
  <Card title={m.dashboard_vpn()}>
    <VPNTunnelPanel status={vpn} control={tunnel} canEnable={(snapshot) => snapshot.state === 'configured'} />
    <Button variant="link" size="sm" class="self-start px-0" href={href('vpn')}>{m.dashboard_vpn_settings()}</Button>
  </Card>
  <Card title={m.dashboard_daemon()}>
    <LoadState status={health.status} retry={health.retry}>
      <dl class="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2">
        <dt class="text-muted-foreground">{m.dashboard_status()}</dt>
        <dd><Status kind="ok">{m.status_ok()}</Status></dd>
      </dl>
    </LoadState>
  </Card>

  <Card title={m.dashboard_resources()}>
    <LoadState status={resources.status} retry={resources.retry} stale={resources.data !== undefined}>
      {#if resources.data}
        {@const info = resources.data}
        <dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2">
          <dt class="text-muted-foreground">{m.dashboard_cpu_load()}</dt>
          <dd>{formatLoad([info.load[0]])}</dd>
          <dt class="text-muted-foreground">{m.system_memory()}</dt>
          <dd>
            {#if info.memory.total_bytes === 0}
              <Status kind="neutral">{m.dashboard_unavailable()}</Status>
            {:else}
              {m.system_used_of({
                used: formatBytes(Math.max(0, info.memory.total_bytes - info.memory.available_bytes)),
                total: formatBytes(info.memory.total_bytes),
              })}
            {/if}
          </dd>
          <dt class="text-muted-foreground">{m.system_uptime()}</dt>
          <dd>{formatUptime(info.uptime_seconds)}</dd>
          <dt class="text-muted-foreground">{m.dashboard_version()}</dt>
          <dd class="break-all">{info.app.version}</dd>
        </dl>
        {#if resources.status.state === 'ready'}
          <Button variant="outline" size="sm"
            type="button"
            class="w-fit text-sm"
            onclick={resources.retry}
          >
            {m.dashboard_refresh()}
          </Button>
        {/if}
      {/if}
    </LoadState>
  </Card>
  <Card title={m.dashboard_throughput()}>
    <LoadState status={throughput.status} retry={throughput.retry} stale={throughput.data !== undefined}>
      {#if throughput.data?.interfaces.length}
        <Table.Root class="w-full table-fixed text-left">
          <Table.Header class="text-muted-foreground">
            <Table.Row>
              <Table.Head scope="col" class="font-normal">{m.dashboard_interface()}</Table.Head>
              <Table.Head scope="col" class="text-right font-normal">{m.dashboard_rx()}</Table.Head>
              <Table.Head scope="col" class="text-right font-normal">{m.dashboard_tx()}</Table.Head>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {#each throughput.data.interfaces as row (row.name)}
              <Table.Row class="border-t border-border">
                <Table.Head scope="row" class="break-all py-2 font-medium">{row.name}</Table.Head>
                <Table.Cell class="py-2 text-right tabular-nums">{formatRate(row.rx_bytes_per_second)}</Table.Cell>
                <Table.Cell class="py-2 text-right tabular-nums">{formatRate(row.tx_bytes_per_second)}</Table.Cell>
              </Table.Row>
            {/each}
          </Table.Body>
        </Table.Root>
      {:else}
        <p class="text-muted-foreground">{m.dashboard_no_interfaces()}</p>
      {/if}
    </LoadState>
  </Card>
</div>
