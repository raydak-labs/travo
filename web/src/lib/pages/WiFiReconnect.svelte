<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import type { ScanResult, WiFiProfileSummary, WiFiReconnectStatus } from '../api/gen';
import { m } from '../paraglide/messages.js';
import type { LoadStatus } from '../ui/loader.svelte';
import Notice from '../ui/Notice.svelte';
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
  {#snippet selectSuggestion()}
    <Button size="sm" type="button" disabled={joinBusy}
      aria-label={suggestion ? m.wifi_saved_select_ssid({ ssid: suggestion.ssid }) : undefined}
      onclick={() => suggestion && select(suggestion)}>{m.wifi_saved_select()}</Button>
  {/snippet}
  <Notice tone="warn" class="mb-3" title={m.wifi_reconnect_exhausted({ attempts: data.attempts })}
    actions={suggestion ? selectSuggestion : undefined}>
    {#if suggestion}
      <span class="break-all">{m.wifi_reconnect_suggest({ ssid: suggestion.ssid })}</span>
    {:else}
      {scan ? m.wifi_reconnect_none_in_range() : m.wifi_reconnect_scan_hint()}
    {/if}
  </Notice>
{/if}
