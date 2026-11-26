<script lang="ts">
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import type { ScanResult, WiFiProfileSummary, WiFiReconnectStatus } from '../api/gen';
import { m } from '../paraglide/messages.js';
import type { LoadStatus } from '../ui/loader.svelte';
import Status from '../ui/Status.svelte';
import { suggestUplink } from '../wifi-reconnect';

let {
  status,
  data,
  profiles,
  scan,
  joinBusy = false,
  select,
}: {
  status: LoadStatus;
  data: WiFiReconnectStatus | undefined;
  profiles: WiFiProfileSummary[];
  scan: ScanResult | undefined;
  joinBusy?: boolean;
  select: (profile: WiFiProfileSummary) => void;
} = $props();

const suggestion = $derived(suggestUplink(profiles, scan));
</script>

{#if status.state === 'error' && !data}
  <p class="mb-3"><Status kind="warn">{m.wifi_reconnect_unavailable()}</Status></p>
{:else if data?.state === 'retrying'}
  <p class="mb-3" role="status">
    <Status kind="warn">{m.wifi_reconnect_retrying({ attempts: data.attempts, max: data.max_attempts })}</Status>
  </p>
{:else if data?.state === 'exhausted'}
  <Alert class="mb-3 grid gap-2" role="alert">
    <p class="font-medium">{m.wifi_reconnect_exhausted({ attempts: data.attempts })}</p>
    {#if suggestion}
      <div class="flex flex-wrap items-center gap-2">
        <span class="break-all text-sm">{m.wifi_reconnect_suggest({ ssid: suggestion.ssid })}</span>
        <Button size="sm" type="button" disabled={joinBusy}
          aria-label={m.wifi_saved_select_ssid({ ssid: suggestion.ssid })}
          onclick={() => suggestion && select(suggestion)}>{m.wifi_saved_select()}</Button>
      </div>
    {:else}
      <p class="text-sm">{scan ? m.wifi_reconnect_none_in_range() : m.wifi_reconnect_scan_hint()}</p>
    {/if}
  </Alert>
{/if}
