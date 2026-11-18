<script lang="ts">
import * as AlertDialog from '$lib/components/ui/alert-dialog';
import { m } from '../paraglide/messages.js';
import type { Confirm, ConfirmRequest } from './confirm.svelte';

// Renders the owner's pending confirmation. Cancel is first in the DOM, so it takes focus.
let { confirm }: { confirm: Confirm } = $props();
// The last question stays rendered while the dialog closes, so it never flashes empty.
let request = $state<ConfirmRequest>();
$effect.pre(() => {
  if (confirm.request) request = confirm.request;
});
</script>

<AlertDialog.Root
  open={confirm.request !== undefined}
  onOpenChange={(open) => {
    if (!open) confirm.answer(false);
  }}
>
  <AlertDialog.Content
    portalProps={{ disabled: typeof window === 'undefined' }}
    class="w-[calc(100%-2rem)] border border-border bg-surface text-fg"
  >
    {#if request}
      <AlertDialog.Header>
        <AlertDialog.Title level={2} class="text-lg font-semibold">{request.title}</AlertDialog.Title>
        <AlertDialog.Description>{request.description}</AlertDialog.Description>
      </AlertDialog.Header>
      <AlertDialog.Footer>
        <AlertDialog.Cancel>{m.dialog_cancel()}</AlertDialog.Cancel>
        <AlertDialog.Action
          variant={request.tone === 'danger' ? 'destructive' : 'default'}
          onclick={() => confirm.answer(true)}>{request.action}</AlertDialog.Action
        >
      </AlertDialog.Footer>
    {/if}
  </AlertDialog.Content>
</AlertDialog.Root>
