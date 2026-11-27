<script lang="ts">
import ChevronRight from '@lucide/svelte/icons/chevron-right';
import Lock from '@lucide/svelte/icons/lock';
import LockOpen from '@lucide/svelte/icons/lock-open';
import Radio from '@lucide/svelte/icons/radio';
import Signal from '@lucide/svelte/icons/signal';
import Button from '$lib/components/ui/button/button.svelte';
import * as Collapsible from '$lib/components/ui/collapsible';
import type { AccessPoint, Network, ScanResult } from '../api/gen';
import { m } from '../paraglide/messages.js';
import Status from '../ui/Status.svelte';
import { accessPointKey, bandLabel, bandsBadge, securityLabel, signalLabel } from '../wifi';

let {
  result,
  join,
  busy = false,
}: { result: ScanResult; join?: (network: Network) => void; busy?: boolean } = $props();
</script>

{#snippet accessPoints(aps: AccessPoint[])}
  <ul class="space-y-1.5 text-sm text-muted-foreground">
    {#each aps as ap, index (accessPointKey(ap, index))}
      <li class="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-md bg-muted/40 px-2.5 py-1.5">
        <span class="font-mono text-xs text-foreground break-all">{ap.bssid}</span>
        <span class="inline-flex items-center gap-1"><Radio aria-hidden="true" class="size-3.5 shrink-0" />{bandLabel(ap.band)} · {m.wifi_channel({ channel: ap.channel })}</span>
        <span class="inline-flex items-center gap-1 tabular-nums"><Signal aria-hidden="true" class="size-3.5 shrink-0" />{signalLabel(ap.signal_dbm)}</span>
        <span class="inline-flex items-center gap-1">
          {#if ap.security === 'open'}<LockOpen aria-hidden="true" class="size-3.5 shrink-0" />{:else}<Lock aria-hidden="true" class="size-3.5 shrink-0" />{/if}{securityLabel(ap.security)}
        </span>
      </li>
    {/each}
  </ul>
{/snippet}

{#snippet chevron()}
  <ChevronRight aria-hidden="true" class="size-4 shrink-0 text-muted-foreground transition-transform duration-150 group-data-[state=open]:rotate-90" />
{/snippet}

<div>
  {#each result.networks as network (network.ssid)}
    <Collapsible.Root class="border-t border-border py-2 first:border-t-0">
      <div class="flex items-center justify-between gap-3">
        <Collapsible.Trigger class="group flex min-w-0 flex-1 cursor-pointer flex-wrap items-center gap-2 rounded-md px-2 py-1.5 text-left transition-colors hover:bg-muted/50">
          {@render chevron()}
          <span class="font-medium break-all">{network.ssid}</span>
          <Status kind="info">{bandsBadge(network.bands)}</Status>
          <span class="text-sm text-muted-foreground">{signalLabel(network.best_signal_dbm)} · {securityLabel(network.security)}</span>
        </Collapsible.Trigger>
        {#if join}
          <Button variant="outline" size="sm"
            type="button"
            class="shrink-0"
            disabled={busy || network.security === 'other'}
            aria-label={network.security === 'other' ? m.wifi_join_unsupported() : m.wifi_join_network({ ssid: network.ssid })}
            onclick={() => join?.(network)}
          >{network.security === 'other' ? m.wifi_join_unsupported() : m.wifi_join_action()}</Button>
        {/if}
      </div>
      <Collapsible.Content forceMount class="overflow-hidden pt-2 pl-6">
        {@render accessPoints(network.access_points)}
      </Collapsible.Content>
    </Collapsible.Root>
  {/each}

  {#if result.networks.length === 0 && result.hidden.length === 0 && result.radios.every((radio) => radio.status === 'ok')}
    <p class="text-muted-foreground">{m.wifi_no_networks()}</p>
  {/if}

  <div class="mt-2 space-y-2">
    {#each result.radios as radio (radio.radio)}
      {#if radio.status === 'error'}
        <div>
          <Status kind="danger">
            {radio.code === 'scan_timeout'
              ? m.wifi_radio_scan_timeout({ band: bandLabel(radio.band) })
              : m.wifi_radio_scan_failed({ band: bandLabel(radio.band) })}
          </Status>
        </div>
      {:else if radio.status === 'skipped'}
        <div><Status kind="neutral">{m.wifi_radio_disabled({ band: bandLabel(radio.band) })}</Status></div>
      {/if}
    {/each}
  </div>

  {#if result.hidden.length > 0}
    <Collapsible.Root class="border-t border-border py-2">
      <Collapsible.Trigger class="group flex w-full cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-left font-medium transition-colors hover:bg-muted/50">
        {@render chevron()}{m.wifi_hidden({ count: result.hidden.length })}
      </Collapsible.Trigger>
      <Collapsible.Content forceMount class="overflow-hidden pt-2 pl-6">
        {@render accessPoints(result.hidden)}
      </Collapsible.Content>
    </Collapsible.Root>
  {/if}
</div>
