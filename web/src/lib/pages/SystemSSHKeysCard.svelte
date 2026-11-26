<script lang="ts">
import KeySquare from '@lucide/svelte/icons/key-square';
import Plus from '@lucide/svelte/icons/plus';
import Trash2 from '@lucide/svelte/icons/trash-2';
import Button from '$lib/components/ui/button/button.svelte';
import * as Dialog from '$lib/components/ui/dialog';
import Label from '$lib/components/ui/label/label.svelte';
import * as Table from '$lib/components/ui/table';
import Textarea from '$lib/components/ui/textarea/textarea.svelte';
import type { PublicKey } from '../api/gen';
import { addSSHKey, fetchSSHKeys, pastedKey, removeSSHKey } from '../api/ssh-keys';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import EmptyState from '../ui/EmptyState.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Section from '../ui/Section.svelte';

const keys = createLoader(fetchSSHKeys);
const confirm = createConfirm();
let text = $state('');
let busy = $state(false);
let invalid = $state(false);

function label(key: PublicKey): string {
  return key.comment || key.type;
}

async function add(event: SubmitEvent) {
  event.preventDefault();
  if (busy) return;
  const line = pastedKey(text);
  invalid = line === undefined;
  if (!line) return;
  busy = true;
  const result = await addSSHKey(line);
  busy = false;
  if ('error' in result) {
    notify.error(result.error);
    return;
  }
  text = '';
  adding = false;
  notify.success(m.ssh_added({ name: label(result.key) }));
  void keys.retry();
}

async function remove(key: PublicKey) {
  const ok = await confirm.ask({
    title: m.ssh_remove_title({ name: label(key) }),
    description: m.ssh_remove_body(),
    action: m.ssh_remove(),
    tone: 'danger',
  });
  if (!ok) return;
  busy = true;
  const result = await removeSSHKey(key.id);
  busy = false;
  if ('error' in result) notify.error(result.error);
  else notify.success(m.ssh_removed({ name: label(key) }));
  void keys.retry();
}
const formId = $props.id();
let adding = $state(false);
</script>

<Section title={m.ssh_title()} description={m.ssh_intro()}>
  {#snippet actions()}
    <Button variant="outline" size="sm" type="button" disabled={busy} onclick={() => { text = ''; invalid = false; adding = true; }}><Plus aria-hidden="true" />{m.ssh_add()}</Button>
  {/snippet}
  <LoadState status={keys.status} retry={keys.retry} stale={keys.data !== undefined}>
    {#if keys.data}
      {#if keys.data.keys.length === 0}
        <EmptyState icon={KeySquare} title={m.ssh_empty()} />
      {:else}
        <Table.Root aria-label={m.ssh_title()}>
          <Table.Header>
            <Table.Row>
              <Table.Head>{m.ssh_key()}</Table.Head>
              <Table.Head class="max-sm:hidden">{m.ssh_fingerprint()}</Table.Head>
              <Table.Head class="w-12"><span class="sr-only">{m.table_actions()}</span></Table.Head>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {#each keys.data.keys as key (key.id)}
              <Table.Row>
                <Table.Cell class="font-medium break-all whitespace-normal">{label(key)}</Table.Cell>
                <Table.Cell class="font-mono text-xs break-all whitespace-normal text-muted-foreground max-sm:hidden">{key.type} {key.fingerprint}</Table.Cell>
                <Table.Cell class="text-right">
                  <Button variant="ghost" size="icon" type="button" disabled={busy}
                    aria-label={m.ssh_remove_name({ name: label(key) })}
                    onclick={() => remove(key)}><Trash2 aria-hidden="true" size={16} /></Button>
                </Table.Cell>
              </Table.Row>
            {/each}
          </Table.Body>
        </Table.Root>
      {/if}
      {#if keys.data.unmanaged > 0}
        <p class="text-sm text-muted-foreground">{m.ssh_unmanaged({ count: keys.data.unmanaged })}</p>
      {/if}
    {/if}
  </LoadState>
</Section>

<Dialog.Root bind:open={adding}>
  <Dialog.Content portalProps={{ disabled: typeof window === 'undefined' }}>
    <Dialog.Header>
      <Dialog.Title level={2}>{m.ssh_add()}</Dialog.Title>
      <Dialog.Description>{m.ssh_intro()}</Dialog.Description>
    </Dialog.Header>
    <form id={formId} class="grid gap-2" onsubmit={add}>
      <Label for="{formId}-key">{m.ssh_add_label()}</Label>
      <Textarea id="{formId}-key" bind:value={text} rows={4} spellcheck={false} autocomplete="off" disabled={busy}
        class="font-mono text-xs" placeholder="ssh-ed25519 AAAA… you@laptop" aria-invalid={invalid} />
      {#if invalid}<p class="text-sm text-danger" role="alert">{m.ssh_err_invalid()}</p>{/if}
    </form>
    <Dialog.Footer>
      <Button variant="outline" type="button" onclick={() => { adding = false; }}>{m.dialog_cancel()}</Button>
      <Button type="submit" form={formId} disabled={busy || !text.trim()}>{m.ssh_add()}</Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

<ConfirmDialog {confirm} />
