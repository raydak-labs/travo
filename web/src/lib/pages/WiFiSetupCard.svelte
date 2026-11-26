<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import type { TxnBody } from '../api/gen';
import { fetchAccessPoints, setupAccessPoints } from '../api/wifi';
import { m } from '../paraglide/messages.js';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import Section from '../ui/Section.svelte';
import { wifiNotSetUp } from '../wifi';

type Send = () => Promise<{ data?: TxnBody; error?: { code?: string } }>;

// Shown while the router's own WiFi is still as OpenWrt ships it (off and open): the first
// thing a travel router needs. One name and key for every band; the page owns the transaction.
let { busy = false, apply }: { busy?: boolean; apply: (label: string, send: Send) => void } =
  $props();

const aps = createLoader(fetchAccessPoints);
let ssid = $state('Travo');
let key = $state('');

function save(event: SubmitEvent) {
  event.preventDefault();
  if (busy) return;
  const body = { ssid, key };
  apply(ssid, () => setupAccessPoints(body));
  key = '';
}
</script>

{#if aps.status.state === 'error' && !aps.data}
  <Section title={m.wifi_setup_title()}>
    <LoadState status={aps.status} retry={aps.retry}>{null}</LoadState>
  </Section>
{:else if aps.data && wifiNotSetUp(aps.data)}
  <Section title={m.wifi_setup_title()} description={m.wifi_setup_description()}>
    <form class="grid gap-4 sm:max-w-md" onsubmit={save}>
      <Label class="grid gap-1">
        {m.wifi_ap_ssid()}
        <Input bind:value={ssid} required maxlength={32} disabled={busy} />
      </Label>
      <Label class="grid gap-1">
        {m.wifi_ap_key()}
        <Input bind:value={key} type="password" autocomplete="new-password" required minlength={8} maxlength={63} disabled={busy} />
        <span class="text-sm text-muted-foreground">{m.wifi_setup_key_hint()}</span>
      </Label>
      <div>
        <Button type="submit" disabled={busy}>{m.wifi_setup_submit()}</Button>
      </div>
    </form>
  </Section>
{/if}
