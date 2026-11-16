<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import * as Dialog from '$lib/components/ui/dialog';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import type { JoinWiFiData, Network } from '../api/gen';
import { m } from '../paraglide/messages.js';
import Status from '../ui/Status.svelte';

let {
  network,
  busy = false,
  submit,
  save,
  close,
  returnFocus = null,
}: {
  network: Network;
  busy?: boolean;
  submit: (body: JoinWiFiData['body']) => void;
  /** Stores the credentials without touching the uplink; resolves to an error message or nothing. */
  save?: (body: JoinWiFiData['body']) => Promise<string | undefined>;
  close: () => void;
  returnFocus?: HTMLElement | null;
} = $props();
let open = $state(true);
let key = $state('');
let saving = $state(false);
let saveError = $state('');
let form = $state<HTMLFormElement | null>(null);

function body(): JoinWiFiData['body'] {
  return { ssid: network.ssid, security: network.security, key };
}

function onSubmit(event: SubmitEvent) {
  event.preventDefault();
  if (busy || saving) return;
  submit(body());
  open = false;
  key = '';
}

async function onSave() {
  if (!save || busy || saving || !form?.reportValidity()) return;
  saving = true;
  saveError = '';
  // On failure the key stays in this open dialog only, so the operator can retry.
  const error = await save(body());
  saving = false;
  if (error) {
    saveError = error;
    return;
  }
  open = false;
  key = '';
}

function onClose() {
  key = '';
  close();
}
</script>

<Dialog.Root bind:open onOpenChange={(value) => { if (!value) key = ''; }}>
<Dialog.Content portalProps={{ disabled: typeof window === "undefined" }} showCloseButton={false}
 onInteractOutside={(event) => event.preventDefault()}
 onCloseAutoFocus={(event) => { event.preventDefault(); if (returnFocus instanceof HTMLElement) returnFocus.focus(); onClose(); }} aria-label={m.wifi_join_dialog()} class="w-[min(90vw,26rem)] rounded-lg border border-border bg-surface p-5 text-fg">
  <form class="space-y-4" onsubmit={onSubmit} bind:this={form}>
    <Dialog.Title level={2} class="break-all text-lg font-semibold">{network.ssid}</Dialog.Title>
    {#if network.security !== 'open'}
      <Label class="grid gap-1">
        {m.wifi_join_key()}
        <Input
          bind:value={key}
          type="password"
          autocomplete="off"
          required
          minlength={8}
          maxlength={63}
          disabled={busy || saving}
        />
      </Label>
    {/if}
    {#if saveError}
      <div role="alert"><Status kind="danger">{saveError}</Status></div>
    {/if}
    <div class="flex flex-wrap justify-end gap-2">
      <Button variant="outline" type="button" onclick={() => open = false}>
        {m.wifi_join_cancel()}
      </Button>
      {#if save}
        <Button variant="outline" type="button" disabled={busy || saving} aria-busy={saving} onclick={onSave}>
          {m.wifi_save_action()}
        </Button>
      {/if}
      <Button variant="outline" type="submit" disabled={busy || saving}>
        {m.wifi_join_action()}
      </Button>
    </div>
  </form>
</Dialog.Content>
</Dialog.Root>
