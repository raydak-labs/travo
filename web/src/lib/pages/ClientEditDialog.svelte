<script lang="ts">
import { untrack } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import * as Dialog from '$lib/components/ui/dialog';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import { clientName, type LanClient, type Reservation } from '../api/clients';
import { type ClientEdit, planClientEdit } from '../clients';
import { m } from '../paraglide/messages.js';
import Status from '../ui/Status.svelte';

let {
  client,
  reservation,
  save,
  close,
}: {
  client: LanClient;
  reservation?: Reservation;
  /** Applies the edit; resolves to an error message, or nothing when it saved. */
  save: (edit: ClientEdit) => Promise<string | undefined>;
  close: () => void;
} = $props();

let open = $state(true);
// The dialog edits a snapshot: a list refresh while it is open must not reset the fields.
const initial = untrack(() => $state.snapshot({ client, reservation }));
let alias = $state(initial.client.alias ?? '');
let reserved = $state(initial.reservation !== undefined);
let ip = $state(initial.reservation?.ip ?? initial.client.ip ?? '');
let hostname = $state(initial.reservation?.name ?? '');
let saving = $state(false);
let failure = $state('');

async function onSubmit(event: SubmitEvent) {
  event.preventDefault();
  if (saving) return;
  const edit = planClientEdit(initial.client, initial.reservation, {
    alias,
    reserved,
    ip,
    name: hostname,
  });
  if (edit.steps.length === 0) {
    open = false;
    return;
  }
  saving = true;
  failure = '';
  const error = await save(edit);
  saving = false;
  if (error) failure = error;
  else open = false;
}
</script>

<Dialog.Root bind:open>
<Dialog.Content portalProps={{ disabled: typeof window === "undefined" }} showCloseButton={false}
 onCloseAutoFocus={(event) => { event.preventDefault(); close(); }}
 class="w-[min(90vw,26rem)] rounded-lg border border-border bg-surface p-5 text-fg">
  <form class="space-y-4" onsubmit={onSubmit}>
    <Dialog.Title level={2} class="break-all text-lg font-semibold">{m.clients_edit_name({ name: clientName(initial.client) })}</Dialog.Title>
    <Dialog.Description class="font-mono text-sm text-muted-foreground">{initial.client.mac}</Dialog.Description>
    <Label class="grid gap-1">
      {m.clients_alias()}
      <Input bind:value={alias} maxlength={64} autocomplete="off" placeholder={m.clients_alias_hint()} disabled={saving} />
    </Label>
    <label class="flex items-center gap-2 text-sm">
      <input type="checkbox" bind:checked={reserved} disabled={saving} />
      {m.clients_reserve_label()}
    </label>
    {#if reserved}
      <Label class="grid gap-1">
        {m.clients_reserve_ip()}
        <Input bind:value={ip} required inputmode="decimal" pattern={'\\d{1,3}(\\.\\d{1,3}){3}'} placeholder="192.168.1.50" disabled={saving} />
      </Label>
      <Label class="grid gap-1">
        {m.clients_reserve_host()}
        <Input bind:value={hostname} maxlength={63} pattern={'[A-Za-z0-9]([A-Za-z0-9\\-]*[A-Za-z0-9])?'} autocomplete="off" disabled={saving} />
      </Label>
    {/if}
    {#if failure}
      <div role="alert"><Status kind="danger">{failure}</Status></div>
    {/if}
    <div class="flex flex-wrap justify-end gap-2">
      <Button variant="outline" type="button" onclick={() => { open = false; }}>{m.clients_cancel()}</Button>
      <Button variant="default" type="submit" disabled={saving} aria-busy={saving}>{m.clients_save()}</Button>
    </div>
  </form>
</Dialog.Content>
</Dialog.Root>
