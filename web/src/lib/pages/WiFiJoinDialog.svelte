<script lang="ts">
import type { JoinWiFiData, Network } from '../api/gen';
import { m } from '../paraglide/messages.js';

let {
  network,
  busy = false,
  submit,
  close,
}: {
  network: Network;
  busy?: boolean;
  submit: (body: JoinWiFiData['body']) => void;
  close: () => void;
} = $props();
let dialog: HTMLDialogElement;
let key = $state('');

$effect(() => {
  dialog?.showModal();
});

function onSubmit(event: SubmitEvent) {
  event.preventDefault();
  if (busy) return;
  submit({ ssid: network.ssid, security: network.security, key });
  dialog.close();
  key = '';
}

function onClose() {
  key = '';
  close();
}
</script>

<dialog
  bind:this={dialog}
  onclose={onClose}
  aria-label={m.wifi_join_dialog()}
  class="m-auto w-[min(90vw,26rem)] rounded-lg border border-border bg-panel p-5 text-fg backdrop:bg-black/50"
>
  <form class="space-y-4" onsubmit={onSubmit}>
    <h2 class="break-all text-lg font-semibold">{network.ssid}</h2>
    {#if network.security !== 'open'}
      <label class="grid gap-1">
        {m.wifi_join_key()}
        <input
          bind:value={key}
          type="password"
          autocomplete="off"
          required
          minlength="8"
          maxlength="63"
          disabled={busy}
          class="rounded border border-border bg-bg px-3 py-2"
        />
      </label>
    {/if}
    <div class="flex justify-end gap-2">
      <button type="button" class="rounded border border-border px-3 py-2" onclick={() => dialog.close()}>
        {m.wifi_join_cancel()}
      </button>
      <button type="submit" class="rounded border border-border px-3 py-2 disabled:opacity-60" disabled={busy}>
        {m.wifi_join_action()}
      </button>
    </div>
  </form>
</dialog>
