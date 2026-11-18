<script lang="ts">
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import * as Dialog from '$lib/components/ui/dialog';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import type { ApiTokenCreated } from '../api/gen';
import { copyText } from '../clipboard';
import { m } from '../paraglide/messages.js';

// Shows a freshly created token's secret exactly once. Closing drops it from memory.
let { created, close }: { created: ApiTokenCreated | undefined; close: () => void } = $props();

let field: HTMLInputElement | null = $state(null);
let copied = $state<boolean>();

async function copy() {
  if (!created) return;
  copied = await copyText(created.token, {
    clipboard: typeof navigator === 'undefined' ? undefined : navigator.clipboard,
    selectAndCopy: () => {
      field?.select();
      return document.execCommand('copy');
    },
  });
}

function onOpenChange(open: boolean) {
  if (open) return;
  copied = undefined;
  close();
}
</script>

<Dialog.Root open={created !== undefined} {onOpenChange}>
  <Dialog.Content portalProps={{ disabled: typeof window === 'undefined' }} showCloseButton={false}
    class="w-[min(90vw,30rem)] rounded-lg border border-border bg-surface p-5 text-fg">
    <Dialog.Title level={2} class="text-lg font-semibold">{m.tokens_secret_title()}</Dialog.Title>
    <Dialog.Description>{m.tokens_secret_once()}</Dialog.Description>
    {#if created}
      <Label class="grid gap-1">
        {m.tokens_secret_label({ name: created.name })}
        <Input bind:ref={field} value={created.token} readonly class="font-mono text-sm"
          autocomplete="off" spellcheck={false} onfocus={() => field?.select()} />
      </Label>
    {/if}
    {#if copied === false}
      <Alert class="status status-danger">{m.tokens_copy_failed()}</Alert>
    {/if}
    <div class="flex justify-end gap-2">
      <Button variant="outline" type="button" onclick={copy}>
        {copied ? m.tokens_copied() : m.tokens_copy()}
      </Button>
      <Button variant="default" type="button" onclick={() => onOpenChange(false)}>{m.tokens_done()}</Button>
    </div>
  </Dialog.Content>
</Dialog.Root>
