<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import * as Dialog from '$lib/components/ui/dialog';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import type { JoinWiFiData, Network } from '../api/gen';
import { m } from '../paraglide/messages.js';

let {
  network,
  busy = false,
  submit,
  close,
  returnFocus = null,
}: {
  network: Network;
  busy?: boolean;
  submit: (body: JoinWiFiData['body']) => void;
  close: () => void;
  returnFocus?: HTMLElement | null;
} = $props();
let open = $state(true);
let key = $state('');

function onSubmit(event: SubmitEvent) {
  event.preventDefault();
  if (busy) return;
  submit({ ssid: network.ssid, security: network.security, key });
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
  <form class="space-y-4" onsubmit={onSubmit}>
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
          disabled={busy}
        />
      </Label>
    {/if}
    <div class="flex justify-end gap-2">
      <Button variant="outline" type="button" onclick={() => open = false}>
        {m.wifi_join_cancel()}
      </Button>
      <Button variant="outline" type="submit" disabled={busy}>
        {m.wifi_join_action()}
      </Button>
    </div>
  </form>
</Dialog.Content>
</Dialog.Root>
