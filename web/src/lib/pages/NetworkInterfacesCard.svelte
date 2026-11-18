<script lang="ts">
import { onDestroy } from 'svelte';
import {
  fetchInterfaces,
  fetchZones,
  interfaceAddresses,
  interfaceStatus,
} from '../api/network-view';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import Status from '../ui/Status.svelte';

// Read-only view; edits stay in LuCI. Uptime moves, so the list refreshes.
const ifaces = createLoader(fetchInterfaces, 10000);
const zones = createLoader(fetchZones, 30000);
onDestroy(() => {
  ifaces.stop();
  zones.stop();
});
</script>

<Card title={m.netview_interfaces()}>
  <p class="text-muted-foreground">{m.netview_intro()}</p>
  <LoadState status={ifaces.status} retry={ifaces.retry} stale={ifaces.data !== undefined}>
    {#if ifaces.data?.partial}
      <p class="mt-2" role="note"><Status kind="warn">{m.netview_partial()}</Status></p>
    {/if}
    {#if ifaces.data}
      <ul class="mt-3 divide-y divide-border" aria-label={m.netview_interfaces()}>
        {#each ifaces.data.interfaces as it (it.name)}
          {@const state = interfaceStatus(it)}
          {@const addrs = interfaceAddresses(it)}
          <li class="flex flex-col gap-1 py-3">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-medium">{it.name}</span>
              <Status kind={state.kind}>{state.label}</Status>
              {#if it.uplink}<Status kind="info">{m.netview_uplink()}</Status>{/if}
              {#if it.zone}<span class="text-sm text-muted-foreground">{m.netview_zone({ zone: it.zone })}</span>{/if}
            </div>
            <p class="break-all font-mono text-sm text-muted-foreground">
              {it.proto}{#if it.device} · {it.device}{/if}{#if addrs} · {addrs}{/if}
            </p>
            {#if it.dns_servers.length}
              <p class="break-all text-sm text-muted-foreground">{m.netview_dns({ servers: it.dns_servers.join(', ') })}</p>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
  </LoadState>
</Card>

<Card title={m.netview_zones()}>
  <LoadState status={zones.status} retry={zones.retry} stale={zones.data !== undefined}>
    {#if zones.data?.length === 0}
      <p class="text-muted-foreground">{m.netview_zones_empty()}</p>
    {:else if zones.data}
      <ul class="divide-y divide-border" aria-label={m.netview_zones()}>
        {#each zones.data as z (z.name)}
          <li class="flex flex-col gap-1 py-3">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-medium">{z.name}</span>
              {#if z.masq}<Status kind="info">{m.netview_masq()}</Status>{/if}
            </div>
            <p class="font-mono text-sm text-muted-foreground">
              {m.netview_policies({ input: z.input, output: z.output, forward: z.forward })}
            </p>
            <p class="break-all text-sm text-muted-foreground">
              {m.netview_networks({ networks: z.networks.join(', ') || '–' })}
              {#if z.forwards_to.length} · {m.netview_forwards({ zones: z.forwards_to.join(', ') })}{/if}
            </p>
          </li>
        {/each}
      </ul>
    {/if}
  </LoadState>
</Card>
