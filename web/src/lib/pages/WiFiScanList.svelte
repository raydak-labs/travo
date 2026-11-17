<script lang="ts">
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
  <ul class="mb-3 space-y-1 pl-4 text-sm text-muted-foreground">
    {#each aps as ap, index (accessPointKey(ap, index))}
      <li>
        <span class="break-all">{ap.bssid}</span> · {bandLabel(ap.band)} · {m.wifi_channel({ channel: ap.channel })} · {signalLabel(ap.signal_dbm)} · {securityLabel(ap.security)}
      </li>
    {/each}
  </ul>
{/snippet}

<div>
  {#each result.networks as network (network.ssid)}
    <Collapsible.Root class="border-t border-border py-2 first:border-t-0">
      <Collapsible.Trigger class="flex cursor-pointer flex-wrap items-center gap-2">
        <span class="break-all font-medium">{network.ssid}</span>
        <Status kind="info">{bandsBadge(network.bands)}</Status>
        <span class="text-sm text-muted-foreground">{signalLabel(network.best_signal_dbm)} · {securityLabel(network.security)}</span>
      </Collapsible.Trigger><Collapsible.Content forceMount>
      {@render accessPoints(network.access_points)}
    </Collapsible.Content></Collapsible.Root>
    {#if join}
      <Button variant="outline" size="sm"
        type="button"
        class="mb-2 text-sm"
        disabled={busy || network.security === 'other'}
        aria-label={network.security === 'other' ? m.wifi_join_unsupported() : m.wifi_join_network({ ssid: network.ssid })}
        onclick={() => join?.(network)}
      >{network.security === 'other' ? m.wifi_join_unsupported() : m.wifi_join_action()}</Button>
    {/if}
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
      <Collapsible.Trigger class="cursor-pointer font-medium">{m.wifi_hidden({ count: result.hidden.length })}</Collapsible.Trigger><Collapsible.Content forceMount>
      {@render accessPoints(result.hidden)}
    </Collapsible.Content></Collapsible.Root>
  {/if}
</div>
