<script lang="ts">
import type { WiFiSummary as Summary } from '../api/gen';
import { m } from '../paraglide/messages.js';
import Status from '../ui/Status.svelte';
import SummaryBand from '../ui/SummaryBand.svelte';
import SummaryTile from '../ui/SummaryTile.svelte';
import { bandLabel } from '../wifi';

let { data }: { data: Summary } = $props();
</script>

<SummaryBand title={m.wifi_summary_title()}>
  <SummaryTile label={m.wifi_uplink()}>
    {#if data.stas.length === 0}
      <div><Status kind="neutral">{m.wifi_no_uplink()}</Status></div>
    {:else}
      <ul class="space-y-2">
        {#each data.stas as sta (sta.section)}
          <li class="flex flex-wrap items-center gap-2">
            <span class="break-all">{sta.ssid}</span>
            {#if sta.associated === null}
              <Status kind="neutral">{m.wifi_unknown()}</Status>
            {:else if sta.associated}
              <Status kind="ok">{m.wifi_connected()}</Status>
            {:else}
              <Status kind="warn">{m.wifi_not_connected()}</Status>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
  </SummaryTile>

  <SummaryTile label={m.wifi_access_points()}>
    {#if data.aps.length === 0}
      <div><Status kind="neutral">{m.wifi_none()}</Status></div>
    {:else}
      <ul class="space-y-2">
        {#each data.aps as ap (ap.section)}
          <li class="flex flex-wrap items-center gap-2">
            <span class="break-all">{ap.ssid}</span>
            {#if ap.up === null}
              <Status kind="neutral">{m.wifi_unknown()}</Status>
            {:else if ap.up}
              <Status kind="ok">{m.wifi_up()}</Status>
            {:else}
              <Status kind="danger">{m.wifi_down()}</Status>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
  </SummaryTile>

  <SummaryTile label={m.wifi_radios_configured()}>
    {#if data.radios.length === 0}
      <div><Status kind="neutral">{m.wifi_none()}</Status></div>
    {:else}
      <ul class="space-y-2">
        {#each data.radios as radio (radio.name)}
          <li class="flex flex-wrap items-center gap-2">
            <span>{radio.name} · {bandLabel(radio.band)}</span>
            {#if radio.disabled}
              <Status kind="neutral">{m.wifi_disabled()}</Status>
            {:else if radio.up}
              <Status kind="ok">{m.wifi_up()}</Status>
            {:else}
              <Status kind="warn">{m.wifi_not_up()}</Status>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
  </SummaryTile>
</SummaryBand>
