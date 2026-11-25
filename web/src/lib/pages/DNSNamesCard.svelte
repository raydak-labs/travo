<script lang="ts">
import Plus from '@lucide/svelte/icons/plus';
import Tag from '@lucide/svelte/icons/tag';
import Trash2 from '@lucide/svelte/icons/trash-2';
import Badge from '$lib/components/ui/badge/badge.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import * as Dialog from '$lib/components/ui/dialog';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import * as Table from '$lib/components/ui/table';
import { addDNSName, type DnsName, fetchDNSNames, removeDNSName } from '../api/dhcp';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import EmptyState from '../ui/EmptyState.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Section from '../ui/Section.svelte';

const names = createLoader(fetchDNSNames);
const confirm = createConfirm();
let name = $state('');
let ip = $state('');
let busy = $state(false);
let invalid = $state(false);

function label(entry: DnsName): string {
  return entry.names.join(' ');
}

async function add(event: SubmitEvent) {
  event.preventDefault();
  if (busy) return;
  invalid = name.trim() === '' || ip.trim() === '';
  if (invalid) return;
  busy = true;
  const result = await addDNSName(name, ip);
  busy = false;
  if ('error' in result) {
    notify.error(result.error);
    return;
  }
  name = '';
  ip = '';
  adding = false;
  notify.success(m.dns_names_added({ name: label(result.entry), ip: result.entry.ip }));
  void names.retry();
}

async function remove(entry: DnsName) {
  const ok = await confirm.ask({
    title: m.dns_names_remove_title({ name: label(entry) }),
    description: m.dns_names_remove_body(),
    action: m.dns_names_remove(),
    tone: 'danger',
  });
  if (!ok) return;
  busy = true;
  const result = await removeDNSName(entry.id);
  busy = false;
  if ('error' in result) notify.error(result.error);
  else notify.success(m.dns_names_removed({ name: label(entry) }));
  void names.retry();
}
const formId = $props.id();
let adding = $state(false);
</script>

<Section title={m.dns_names_title()} description={m.dns_names_intro()}>
  {#snippet actions()}
    <Button variant="outline" size="sm" type="button" disabled={busy} onclick={() => { invalid = false; adding = true; }}><Plus aria-hidden="true" />{m.dns_names_add()}</Button>
  {/snippet}
  <LoadState status={names.status} retry={names.retry} stale={names.data !== undefined}>
    {#if names.data}
      {#if names.data.names.length === 0}
        <EmptyState icon={Tag} title={m.dns_names_empty()} />
      {:else}
        <Table.Root aria-label={m.dns_names_title()}>
          <Table.Header>
            <Table.Row>
              <Table.Head>{m.dns_names_name()}</Table.Head>
              <Table.Head>{m.dns_names_ip()}</Table.Head>
              <Table.Head class="w-12"><span class="sr-only">{m.table_actions()}</span></Table.Head>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {#each names.data.names as entry (entry.id)}
              <Table.Row>
                <Table.Cell class="font-medium break-all whitespace-normal">{label(entry)}</Table.Cell>
                <Table.Cell class="font-mono text-xs">{entry.ip}</Table.Cell>
                <Table.Cell class="text-right">
                  {#if entry.owned}
                    <Button variant="ghost" size="icon" type="button" disabled={busy}
                      aria-label={m.dns_names_remove_name({ name: label(entry) })}
                      onclick={() => remove(entry)}><Trash2 aria-hidden="true" size={16} /></Button>
                  {:else}
                    <Badge variant="outline">{m.dns_names_luci()}</Badge>
                  {/if}
                </Table.Cell>
              </Table.Row>
            {/each}
          </Table.Body>
        </Table.Root>
      {/if}
    {/if}
  </LoadState>
</Section>

<Dialog.Root bind:open={adding}>
  <Dialog.Content portalProps={{ disabled: typeof window === 'undefined' }}>
    <Dialog.Header>
      <Dialog.Title level={2}>{m.dns_names_add()}</Dialog.Title>
      <Dialog.Description>{m.dns_names_intro()}</Dialog.Description>
    </Dialog.Header>
    <form id={formId} class="grid gap-4" onsubmit={add}>
      <div class="grid gap-2">
        <Label for="{formId}-name">{m.dns_names_name()}</Label>
        <Input id="{formId}-name" bind:value={name} maxlength={253} autocomplete="off" autocapitalize="off" spellcheck={false} disabled={busy} placeholder="nas" aria-invalid={invalid} />
      </div>
      <div class="grid gap-2">
        <Label for="{formId}-ip">{m.dns_names_ip()}</Label>
        <Input id="{formId}-ip" bind:value={ip} maxlength={45} autocomplete="off" autocapitalize="off" spellcheck={false} disabled={busy} placeholder="192.168.1.20" class="font-mono" aria-invalid={invalid} />
      </div>
      {#if invalid}<p class="text-sm text-danger" role="alert">{m.dns_names_err_input()}</p>{/if}
    </form>
    <Dialog.Footer>
      <Button variant="outline" type="button" onclick={() => { adding = false; }}>{m.dialog_cancel()}</Button>
      <Button type="submit" form={formId} disabled={busy}>{m.dns_names_add()}</Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

<ConfirmDialog {confirm} />
