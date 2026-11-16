<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import * as Dialog from '$lib/components/ui/dialog';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import type { ApConfig, UpdateWiFiAccessPointData } from '../api/gen';
import { fetchAccessPoints } from '../api/wifi';
import { m } from '../paraglide/messages.js';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import Status from '../ui/Status.svelte';
import { bandLabel } from '../wifi';

let {
  busy = false,
  apply,
}: {
  busy?: boolean;
  apply: (section: string, body: UpdateWiFiAccessPointData['body']) => void;
} = $props();

const aps = createLoader(fetchAccessPoints);
let editing = $state<ApConfig>();
let open = $state(false);
let ssid = $state('');
let key = $state('');

function edit(ap: ApConfig) {
  editing = ap;
  ssid = ap.ssid;
  key = '';
  open = true;
}

function save(event: SubmitEvent) {
  event.preventDefault();
  if (!editing || busy) return;
  apply(editing.section, { ssid, enabled: editing.enabled, ...(key ? { key } : {}) });
  key = '';
  open = false;
}
</script>

<LoadState status={aps.status} retry={aps.retry} stale={aps.data !== undefined}>
  {#if aps.data?.length === 0}
    <Status kind="neutral">{m.wifi_none()}</Status>
  {:else if aps.data}
    <ul class="space-y-3">
      {#each aps.data as ap (ap.section)}
        <li class="flex flex-wrap items-center gap-2">
          <span class="break-all">{ap.ssid}</span>
          <span class="text-sm text-muted-foreground">{bandLabel(ap.band)}</span>
          {#if ap.guest}<Status kind="info">{m.wifi_ap_guest()}</Status>{/if}
          <Status kind={ap.enabled ? 'ok' : 'neutral'}>{ap.enabled ? m.wifi_up() : m.wifi_disabled()}</Status>
          <span class="ml-auto flex gap-2">
            <Button variant="outline" size="sm" type="button" disabled={busy} onclick={() => edit(ap)}>{m.wifi_ap_edit()}</Button>
            <Button variant="outline" size="sm" type="button" disabled={busy}
              onclick={() => apply(ap.section, { ssid: ap.ssid, enabled: !ap.enabled })}>
              {ap.enabled ? m.wifi_ap_disable() : m.wifi_ap_enable()}
            </Button>
          </span>
        </li>
      {/each}
    </ul>
  {/if}
</LoadState>

<Dialog.Root bind:open onOpenChange={(value) => { if (!value) key = ''; }}>
<Dialog.Content portalProps={{ disabled: typeof window === "undefined" }} showCloseButton={false}
 aria-label={m.wifi_ap_edit_dialog()} class="w-[min(90vw,26rem)] rounded-lg border border-border bg-surface p-5 text-fg">
  <form class="space-y-4" onsubmit={save}>
    <Dialog.Title level={2} class="text-lg font-semibold">{m.wifi_ap_edit_dialog()}</Dialog.Title>
    <Label class="grid gap-1">
      {m.wifi_ap_ssid()}
      <Input bind:value={ssid} required maxlength={32} disabled={busy} />
    </Label>
    <Label class="grid gap-1">
      {m.wifi_ap_key()}
      <Input bind:value={key} type="password" autocomplete="new-password" minlength={8} maxlength={63} disabled={busy} />
      <span class="text-sm text-muted-foreground">{m.wifi_ap_key_hint()}</span>
    </Label>
    <div class="flex justify-end gap-2">
      <Button variant="outline" type="button" onclick={() => (open = false)}>{m.wifi_join_cancel()}</Button>
      <Button variant="outline" type="submit" disabled={busy}>{m.wifi_ap_save()}</Button>
    </div>
  </form>
</Dialog.Content>
</Dialog.Root>
