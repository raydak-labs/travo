<script lang="ts">
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import { fetchInterfaces } from '../api/network-view';
import { validWakeMAC, wake, wakeInterfaces } from '../api/wol';
import { m } from '../paraglide/messages.js';
import { notify } from '../ui/notify';
import Section from '../ui/Section.svelte';

let mac = $state('');
let iface = $state('');
let sending = $state(false);
// Best effort: without the interface list the card still wakes on all LAN bridges.
let networks = $state<string[]>([]);
const request = new AbortController();
onDestroy(() => request.abort());
fetchInterfaces(request.signal).then(
  (list) => {
    networks = wakeInterfaces(list.interfaces);
  },
  () => {},
);

const valid = $derived(validWakeMAC(mac));

async function send(event: SubmitEvent) {
  event.preventDefault();
  if (sending) return;
  if (!valid) {
    notify.error(m.wol_err_mac());
    return;
  }
  sending = true;
  const outcome = await wake(mac, iface, request.signal);
  sending = false;
  if (request.signal.aborted) return;
  if ('error' in outcome) notify.error(outcome.error);
  else
    notify.success(
      m.wol_sent({ target: outcome.wake.mac, interfaces: outcome.wake.interfaces.join(', ') }),
    );
}
</script>

<Section title={m.wol_title()} description={m.wol_intro()}>
  <form class="flex flex-wrap items-end gap-3" onsubmit={send}>
    <Label class="grid min-w-40 flex-1 gap-1">
      {m.wol_mac()}
      <Input bind:value={mac} maxlength={17} autocomplete="off" autocapitalize="off" spellcheck={false}
        disabled={sending} placeholder="aa:bb:cc:dd:ee:ff" class="font-mono" />
    </Label>
    {#if networks.length > 1}
      <Label class="grid gap-1">
        {m.wol_interface()}
        <select bind:value={iface} disabled={sending} class="h-9 rounded-md border border-input bg-transparent px-2 text-sm">
          <option value="">{m.wol_all_networks()}</option>
          {#each networks as name (name)}<option value={name}>{name}</option>{/each}
        </select>
      </Label>
    {/if}
    <Button variant="outline" type="submit" disabled={sending || !mac.trim()}>
      {sending ? m.wol_sending() : m.wol_send()}
    </Button>
  </form>
</Section>
