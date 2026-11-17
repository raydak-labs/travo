<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import type { SetWiFiUplinkMacData, TxnBody } from '../api/gen';
import { fetchUplinkMAC, updateUplinkMAC } from '../api/wifi';
import { m } from '../paraglide/messages.js';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import Status from '../ui/Status.svelte';

type Send = () => Promise<{ data?: TxnBody; error?: { code?: string } }>;

let { busy = false, apply }: { busy?: boolean; apply: (label: string, send: Send) => void } =
  $props();

const policy = createLoader(fetchUplinkMAC);
let custom = $state('');

const modeLabel = {
  default: m.wifi_mac_mode_default,
  random: m.wifi_mac_mode_random,
  custom: m.wifi_mac_mode_custom,
};

function set(body: SetWiFiUplinkMacData['body']) {
  apply(m.wifi_mac_title(), () => updateUplinkMAC(body));
}

function submitCustom(event: SubmitEvent) {
  event.preventDefault();
  if (!busy) set({ mode: 'custom', macaddr: custom.trim() });
}
</script>

<LoadState status={policy.status} retry={policy.retry} stale={policy.data !== undefined}>
  {#if policy.status.state === 'ready' && !policy.data}
    <Status kind="neutral">{m.wifi_mac_no_uplink()}</Status>
  {:else if policy.data}
    {@const p = policy.data}
    <p class="text-sm">
      {m.wifi_mac_current({ mode: modeLabel[p.mode](), mac: p.effective ?? m.wifi_unknown() })}
    </p>
    <div class="mt-3 flex flex-wrap gap-2">
      <Button variant="outline" size="sm" type="button" disabled={busy || p.mode === 'default'} onclick={() => set({ mode: 'default' })}>{m.wifi_mac_mode_default()}</Button>
      <Button variant="outline" size="sm" type="button" disabled={busy || p.mode === 'random'} onclick={() => set({ mode: 'random' })}>{m.wifi_mac_mode_random()}</Button>
      <Button variant="outline" size="sm" type="button" disabled={busy} onclick={() => set({ mode: 'clone' })}>{m.wifi_mac_clone()}</Button>
    </div>
    <form class="mt-3 flex flex-wrap items-end gap-2" onsubmit={submitCustom}>
      <Label class="grid gap-1">
        {m.wifi_mac_custom_label()}
        <Input bind:value={custom} required pattern={'[0-9A-Fa-f]{2}(:[0-9A-Fa-f]{2}){5}'} placeholder="02:11:22:33:44:55" class="w-48" disabled={busy} />
      </Label>
      <Button variant="outline" size="sm" type="submit" disabled={busy}>{m.wifi_mac_set()}</Button>
    </form>
  {/if}
</LoadState>
