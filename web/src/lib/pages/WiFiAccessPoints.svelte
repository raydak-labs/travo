<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import * as Dialog from '$lib/components/ui/dialog';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import type { TxnBody } from '../api/gen';
import { fetchAccessPoints, fetchGuest, updateAccessPoint, updateGuest } from '../api/wifi';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import Status from '../ui/Status.svelte';
import { bandLabel } from '../wifi';

type Send = () => Promise<{ data?: TxnBody; error?: { code?: string } }>;

let {
  busy = false,
  apply,
}: {
  busy?: boolean;
  // send(acknowledge) builds the request; the page owns the transaction flow.
  apply: (ssid: string, send: (acknowledge: boolean) => Send) => void;
} = $props();

const aps = createLoader(fetchAccessPoints);
const guest = createLoader(fetchGuest);
// Editing target: an AP section, or 'guest' for the guest network.
let editing = $state<{ section: string; enabled: boolean }>();
let open = $state(false);
let ssid = $state('');
let key = $state('');

function edit(section: string, current: string, enabled: boolean) {
  editing = { section, enabled };
  ssid = current;
  key = '';
  open = true;
}

function send(section: string, body: { ssid: string; enabled: boolean; key?: string }) {
  const request = (acknowledge: boolean) => {
    const full = acknowledge ? { ...body, acknowledge_lockout: true } : body;
    return section === 'guest' ? () => updateGuest(full) : () => updateAccessPoint(section, full);
  };
  apply(body.ssid, request);
}

const confirm = createConfirm();

// Turning an AP off drops its clients; enabling, or editing an off AP, needs no question.
async function toggle(section: string, current: string, enabled: boolean) {
  if (enabled) {
    const ok = await confirm.ask({
      title: m.wifi_ap_disable_title({ ssid: current }),
      description: m.wifi_ap_disable_body({ ssid: current }),
      action: m.wifi_ap_disable(),
      tone: 'danger',
    });
    if (!ok) return;
  }
  send(section, { ssid: current, enabled: !enabled });
}

function save(event: SubmitEvent) {
  event.preventDefault();
  if (!editing || busy) return;
  send(editing.section, { ssid, enabled: editing.enabled, ...(key ? { key } : {}) });
  key = '';
  open = false;
}
</script>

<LoadState status={aps.status} retry={aps.retry} stale={aps.data !== undefined}>
  {#if aps.data?.length === 0}
    <Status kind="neutral">{m.wifi_none()}</Status>
  {:else if aps.data}
    <ul class="space-y-3">
      {#each aps.data.filter((ap) => ap.section !== 'guest') as ap (ap.section)}
        <li class="flex flex-wrap items-center gap-2">
          <span class="break-all">{ap.ssid}</span>
          <span class="text-sm text-muted-foreground">{bandLabel(ap.band)}</span>
          {#if ap.guest}<Status kind="info">{m.wifi_ap_guest()}</Status>{/if}
          <Status kind={ap.enabled ? 'ok' : 'neutral'}>{ap.enabled ? m.wifi_up() : m.wifi_disabled()}</Status>
          <span class="ml-auto flex gap-2">
            <Button variant="outline" size="sm" type="button" disabled={busy} onclick={() => edit(ap.section, ap.ssid, ap.enabled)}>{m.wifi_ap_edit()}</Button>
            <Button variant="outline" size="sm" type="button" disabled={busy}
              onclick={() => toggle(ap.section, ap.ssid, ap.enabled)}>
              {ap.enabled ? m.wifi_ap_disable() : m.wifi_ap_enable()}
            </Button>
          </span>
        </li>
      {/each}
    </ul>
  {/if}
</LoadState>

<section class="mt-4 border-t border-border pt-4">
  <h3 class="text-sm font-semibold">{m.wifi_guest_title()}</h3>
  <LoadState status={guest.status} retry={guest.retry} stale={guest.data !== undefined}>
    {#if guest.data}
      <p class="mt-1 text-sm text-muted-foreground">{m.wifi_guest_hint({ subnet: guest.data.subnet })}</p>
      <div class="mt-2 flex flex-wrap items-center gap-2">
        {#if guest.data.configured}
          {@const g = guest.data}
          <span class="break-all">{g.ssid}</span>
          <Status kind={g.enabled ? 'ok' : 'neutral'}>{g.enabled ? m.wifi_up() : m.wifi_disabled()}</Status>
          <span class="ml-auto flex gap-2">
            <Button variant="outline" size="sm" type="button" disabled={busy} onclick={() => edit('guest', g.ssid, g.enabled)}>{m.wifi_ap_edit()}</Button>
            <Button variant="outline" size="sm" type="button" disabled={busy}
              onclick={() => toggle('guest', g.ssid, g.enabled)}>
              {g.enabled ? m.wifi_ap_disable() : m.wifi_ap_enable()}
            </Button>
          </span>
        {:else}
          <Status kind="neutral">{m.wifi_guest_none()}</Status>
          <Button variant="outline" size="sm" class="ml-auto" type="button" disabled={busy} onclick={() => edit('guest', '', true)}>{m.wifi_guest_setup()}</Button>
        {/if}
      </div>
    {/if}
  </LoadState>
</section>

<Dialog.Root bind:open onOpenChange={(value) => { if (!value) key = ''; }}>
<Dialog.Content portalProps={{ disabled: typeof window === "undefined" }} showCloseButton={false}
 aria-label={m.wifi_ap_edit_dialog()} class="w-[min(90vw,26rem)] rounded-lg border border-border bg-surface p-5 text-fg">
  <form class="space-y-4" onsubmit={save}>
    <Dialog.Title level={2} class="text-lg font-semibold">{editing?.section === 'guest' ? m.wifi_guest_title() : m.wifi_ap_edit_dialog()}</Dialog.Title>
    <Label class="grid gap-1">
      {m.wifi_ap_ssid()}
      <Input bind:value={ssid} required maxlength={32} disabled={busy} />
    </Label>
    <Label class="grid gap-1">
      {m.wifi_ap_key()}
      <Input bind:value={key} type="password" autocomplete="new-password" minlength={8} maxlength={63} disabled={busy}
        required={editing?.section === 'guest' && !guest.data?.configured} />
      <span class="text-sm text-muted-foreground">{m.wifi_ap_key_hint()}</span>
    </Label>
    <div class="flex justify-end gap-2">
      <Button variant="outline" type="button" onclick={() => (open = false)}>{m.wifi_join_cancel()}</Button>
      <Button variant="outline" type="submit" disabled={busy}>{m.wifi_ap_save()}</Button>
    </div>
  </form>
</Dialog.Content>
</Dialog.Root>

<ConfirmDialog {confirm} />
