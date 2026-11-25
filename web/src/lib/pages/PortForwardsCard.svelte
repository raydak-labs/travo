<script lang="ts">
import ArrowRightLeft from '@lucide/svelte/icons/arrow-right-left';
import Plus from '@lucide/svelte/icons/plus';
import Trash2 from '@lucide/svelte/icons/trash-2';
import { onDestroy } from 'svelte';
import Badge from '$lib/components/ui/badge/badge.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import * as Dialog from '$lib/components/ui/dialog';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import * as Table from '$lib/components/ui/table';
import { clientName, fetchClients } from '../api/clients';
import { addPortForward, deletePortForward } from '../api/gen';
import {
  emptyForm,
  type ForwardForm,
  type ForwardProto,
  fetchPortForwards,
  forwardRefusal,
  type PortForward,
  requestFrom,
  targetLabel,
} from '../api/port-forwards';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import EmptyState from '../ui/EmptyState.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import Section from '../ui/Section.svelte';
import SelectField from '../ui/SelectField.svelte';
import { transactionToast } from '../ui/transaction-toast.svelte';
import { controlPhase, createTunnelControl } from '../vpn-tunnel.svelte';

const forwards = createLoader(fetchPortForwards);
// LAN clients only feed the target suggestions; the card works without them.
const lan = createLoader(fetchClients);
const confirm = createConfirm();
let form = $state<ForwardForm>(emptyForm());
// The forward a guarded remove acts on; empty while adding. `txn.busy` serialises.
let removing = '';

// set(true) adds the form's forward, set(false) removes `removing`.
const txn = createTunnelControl(() => forwards.retry(), {
  submit: (add, signal) =>
    add
      ? addPortForward({ body: requestFrom(form), signal })
      : deletePortForward({ path: { id: removing }, signal }),
  refusal: forwardRefusal,
  confirmed: m.port_forwards_confirmed,
  rolledBack: m.port_forwards_rolled_back,
  rollbackFailed: m.port_forwards_rollback_failed,
  failed: m.port_forwards_failed,
  unknown: m.port_forwards_failed,
});
transactionToast(
  'port-forwards',
  () => controlPhase(txn, m.port_forwards_applying()),
  (ok) => void txn.settle(ok),
);
onDestroy(() => txn.stop());

// A kept add clears the form; a refusal or rollback keeps the user's input.
let settledID = '';
$effect(() => {
  const t = txn.transaction;
  if (!t || t.id === settledID || t.state !== 'confirmed') return;
  settledID = t.id;
  if (!removing) form = emptyForm();
});

const protos: [ForwardProto, () => string][] = [
  ['tcp', m.port_forwards_proto_tcp],
  ['udp', m.port_forwards_proto_udp],
  ['both', m.port_forwards_proto_both],
];

function label(f: PortForward): string {
  return f.name || f.id;
}

async function add(event: SubmitEvent) {
  event.preventDefault();
  if (txn.busy) return;
  adding = false;
  const ok = await confirm.ask({
    title: m.port_forwards_add_title({ port: form.externalPort.trim() }),
    description: m.port_forwards_add_body({ target: form.targetIP.trim() }),
    action: m.port_forwards_add(),
  });
  if (!ok) return;
  removing = '';
  void txn.set(true);
}

async function remove(f: PortForward) {
  if (txn.busy) return;
  const ok = await confirm.ask({
    title: m.port_forwards_remove_title({ name: label(f) }),
    description: m.port_forwards_remove_body(),
    action: m.port_forwards_remove(),
    tone: 'danger',
  });
  if (!ok) return;
  removing = f.id;
  void txn.set(false);
}
const formId = $props.id();
let adding = $state(false);
</script>

<Section title={m.port_forwards_rules()} description={m.port_forwards_intro()}>
  {#snippet actions()}
    <Button variant="outline" size="sm" type="button" disabled={txn.busy} onclick={() => { adding = true; }}><Plus aria-hidden="true" />{m.port_forwards_add()}</Button>
  {/snippet}
  <LoadState status={forwards.status} retry={forwards.retry} stale={forwards.data !== undefined}>
    {#if forwards.data}
      {#if forwards.data.length === 0}
        <EmptyState icon={ArrowRightLeft} title={m.port_forwards_empty()} />
      {:else}
        <Table.Root aria-label={m.port_forwards_title()}>
          <Table.Header>
            <Table.Row>
              <Table.Head>{m.port_forwards_name()}</Table.Head>
              <Table.Head>{m.port_forwards_proto()}</Table.Head>
              <Table.Head>{m.port_forwards_external()}</Table.Head>
              <Table.Head>{m.port_forwards_target_ip()}</Table.Head>
              <Table.Head class="w-12"><span class="sr-only">{m.table_actions()}</span></Table.Head>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {#each forwards.data as f (f.id)}
              <Table.Row>
                <Table.Cell class="font-medium break-all whitespace-normal">
                  {label(f)}
                  {#if !f.enabled}<Badge variant="outline" class="ml-2">{m.port_forwards_disabled()}</Badge>{/if}
                </Table.Cell>
                <Table.Cell class="font-mono text-xs">{f.proto}</Table.Cell>
                <Table.Cell class="font-mono text-xs">{f.external_port || '*'}</Table.Cell>
                <Table.Cell class="font-mono text-xs">{targetLabel(f)}</Table.Cell>
                <Table.Cell class="text-right">
                  {#if f.owned}
                    <Button variant="ghost" size="icon" type="button" disabled={txn.busy}
                      aria-label={m.port_forwards_remove_name({ name: label(f) })}
                      onclick={() => remove(f)}><Trash2 aria-hidden="true" size={16} /></Button>
                  {:else}
                    <Badge variant="outline">{m.port_forwards_luci()}</Badge>
                  {/if}
                </Table.Cell>
              </Table.Row>
            {/each}
          </Table.Body>
        </Table.Root>
      {/if}
    {/if}
  </LoadState>
  <p class="text-sm text-muted-foreground">{m.port_forwards_note()}</p>
</Section>

<Dialog.Root bind:open={adding}>
  <Dialog.Content portalProps={{ disabled: typeof window === 'undefined' }}>
    <Dialog.Header>
      <Dialog.Title level={2}>{m.port_forwards_add()}</Dialog.Title>
      <Dialog.Description>{m.port_forwards_intro()}</Dialog.Description>
    </Dialog.Header>
    <form id={formId} class="grid gap-4 sm:grid-cols-2" onsubmit={add}>
      <div class="grid gap-2 sm:col-span-2">
        <Label for="{formId}-name">{m.port_forwards_name()}</Label>
        <Input id="{formId}-name" bind:value={form.name} maxlength={64} autocomplete="off" disabled={txn.busy} placeholder="NAS" />
      </div>
      <div class="grid gap-2">
        <Label>{m.port_forwards_proto()}</Label>
        <SelectField bind:value={form.proto} disabled={txn.busy} class="w-full"
          options={protos.map(([value, text]) => ({ value, label: text() }))} />
      </div>
      <div class="grid gap-2">
        <Label for="{formId}-ext">{m.port_forwards_external()}</Label>
        <Input id="{formId}-ext" bind:value={form.externalPort} required maxlength={11} autocomplete="off" spellcheck={false} disabled={txn.busy} placeholder="8080" class="font-mono" />
      </div>
      <div class="grid gap-2">
        <Label for="{formId}-ip">{m.port_forwards_target_ip()}</Label>
        <Input id="{formId}-ip" bind:value={form.targetIP} required maxlength={15} autocomplete="off" spellcheck={false} disabled={txn.busy} placeholder="192.168.1.20" class="font-mono" list="port-forward-clients" />
      </div>
      <div class="grid gap-2">
        <Label for="{formId}-port">{m.port_forwards_target_port()}</Label>
        <Input id="{formId}-port" bind:value={form.targetPort} maxlength={11} autocomplete="off" spellcheck={false} disabled={txn.busy} placeholder={m.port_forwards_target_port_same()} class="font-mono" />
      </div>
    </form>
    <datalist id="port-forward-clients">
      {#each lan.data?.clients ?? [] as c (c.mac)}
        {#if c.ip}<option value={c.ip}>{clientName(c)}</option>{/if}
      {/each}
    </datalist>
    <Dialog.Footer>
      <Button variant="outline" type="button" onclick={() => { adding = false; }}>{m.dialog_cancel()}</Button>
      <Button type="submit" form={formId} disabled={txn.busy}>{txn.applying ? m.port_forwards_applying() : m.port_forwards_add()}</Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

<ConfirmDialog {confirm} />
