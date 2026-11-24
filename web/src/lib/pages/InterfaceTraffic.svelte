<script lang="ts">
import * as Table from '$lib/components/ui/table';
import type { ThroughputBody } from '../api/gen';
import { formatBytes } from '../format';
import { m } from '../paraglide/messages.js';
import LoadState from '../ui/LoadState.svelte';
import type { LoadStatus } from '../ui/loader.svelte';

// Per-interface rates; uplink rates are on the topology diagram, this lists every interface.
let {
  throughput,
}: { throughput: { status: LoadStatus; data?: ThroughputBody; retry: () => void } } = $props();

function formatRate(rate: number | null): string {
  if (rate === null) return m.dashboard_rate_unknown();
  const rounded = rate < 1024 ? Math.round(rate * 10) / 10 : rate;
  return m.dashboard_bytes_per_second({ bytes: formatBytes(rounded) });
}
</script>

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
