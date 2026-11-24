<script lang="ts">
import KeySquare from '@lucide/svelte/icons/key-square';
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import Label from '$lib/components/ui/label/label.svelte';
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
</script>

<Section title={m.ssh_title()} description={m.ssh_intro()}>
  <LoadState status={keys.status} retry={keys.retry} stale={keys.data !== undefined}>
    {#if keys.data}
      {#if keys.data.keys.length === 0}
        <EmptyState icon={KeySquare} title={m.ssh_empty()} />
      {:else}
        <ul class="divide-y divide-border" aria-label={m.ssh_title()}>
          {#each keys.data.keys as key (key.id)}
            <li class="flex flex-wrap items-center gap-2 py-2">
              <span class="min-w-0 flex-1">
                <span class="block break-all font-medium">{label(key)}</span>
                <span class="block break-all font-mono text-xs text-muted-foreground">{key.type} {key.fingerprint}</span>
              </span>
              <Button variant="destructive" size="sm" type="button" disabled={busy}
                aria-label={m.ssh_remove_name({ name: label(key) })}
                onclick={() => remove(key)}>{m.ssh_remove()}</Button>
            </li>
          {/each}
        </ul>
      {/if}
      {#if keys.data.unmanaged > 0}
        <p class="text-sm text-muted-foreground">{m.ssh_unmanaged({ count: keys.data.unmanaged })}</p>
      {/if}
    {/if}
  </LoadState>
  <form class="flex flex-col gap-3" onsubmit={add}>
    <Label class="grid gap-1">
      {m.ssh_add_label()}
      <Textarea bind:value={text} rows={3} spellcheck={false} autocomplete="off" disabled={busy}
        class="font-mono text-xs" placeholder="ssh-ed25519 AAAA… you@laptop" aria-invalid={invalid} />
    </Label>
    {#if invalid}
      <Alert class="status status-danger self-start" role="alert">{m.ssh_err_invalid()}</Alert>
    {/if}
    <div>
      <Button variant="default" type="submit" disabled={busy}>{m.ssh_add()}</Button>
    </div>
  </form>
</Section>

<ConfirmDialog {confirm} />
