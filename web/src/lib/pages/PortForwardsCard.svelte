<script lang="ts">
import { onDestroy } from 'svelte';
import Badge from '$lib/components/ui/badge/badge.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
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
import Card from '../ui/Card.svelte';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { transactionToast } from '../ui/transaction-toast.svelte';
import { controlPhase, createTunnelControl } from '../vpn-tunnel.svelte';

const forwards = createLoader(fetchPortForwards);
// LAN clients only feed the target suggestions; the card works without them.
const lan = createLoader(fetchClients);
const confirm = createConfirm();
let form = $state<ForwardForm>(emptyForm());
// The forward a guarded remove acts on; empty while adding. `control.busy` serialises.
let removing = '';

// set(true) adds the form's forward, set(false) removes `removing`.
const control = createTunnelControl(() => forwards.retry(), {
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
  () => controlPhase(control, m.port_forwards_applying()),
  (ok) => void control.settle(ok),
);
onDestroy(() => control.stop());

// A kept add clears the form; a refusal or rollback keeps the user's input.
let settledID = '';
$effect(() => {
  const t = control.transaction;
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
  if (control.busy) return;
  const ok = await confirm.ask({
    title: m.port_forwards_add_title({ port: form.externalPort.trim() }),
    description: m.port_forwards_add_body({ target: form.targetIP.trim() }),
    action: m.port_forwards_add(),
  });
  if (!ok) return;
  removing = '';
  void control.set(true);
}

async function remove(f: PortForward) {
  if (control.busy) return;
  const ok = await confirm.ask({
    title: m.port_forwards_remove_title({ name: label(f) }),
    description: m.port_forwards_remove_body(),
    action: m.port_forwards_remove(),
    tone: 'danger',
  });
  if (!ok) return;
  removing = f.id;
  void control.set(false);
}
</script>

<Card title={m.port_forwards_title()}>
  <p class="text-muted-foreground">{m.port_forwards_intro()}</p>
  <LoadState status={forwards.status} retry={forwards.retry} stale={forwards.data !== undefined}>
    {#if forwards.data}
      {#if forwards.data.length === 0}
        <p class="mt-3 text-muted-foreground">{m.port_forwards_empty()}</p>
      {:else}
        <ul class="mt-3 divide-y divide-border" aria-label={m.port_forwards_title()}>
          {#each forwards.data as f (f.id)}
            <li class="flex flex-wrap items-center gap-2 py-2">
              <span class="min-w-0 flex-1">
                <span class="block break-all font-medium">{label(f)}</span>
                <span class="block break-all font-mono text-xs text-muted-foreground">
                  {f.proto} {f.external_port || '*'} → {targetLabel(f)}
                </span>
              </span>
              {#if !f.enabled}
                <Badge variant="outline">{m.port_forwards_disabled()}</Badge>
              {/if}
              {#if f.owned}
                <Button variant="outline" size="sm" type="button" disabled={control.busy}
                  aria-label={m.port_forwards_remove_name({ name: label(f) })}
                  onclick={() => remove(f)}>{m.port_forwards_remove()}</Button>
              {:else}
                <Badge variant="outline">{m.port_forwards_luci()}</Badge>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}
    {/if}
  </LoadState>
  <form class="mt-4 flex flex-wrap items-end gap-3" onsubmit={add}>
    <Label class="grid min-w-32 flex-1 gap-1">
      {m.port_forwards_name()}
      <Input bind:value={form.name} maxlength={64} autocomplete="off" disabled={control.busy} placeholder="NAS" />
    </Label>
    <Label class="grid gap-1">
      {m.port_forwards_proto()}
      <select bind:value={form.proto} disabled={control.busy}
        class="h-9 rounded-md border border-input bg-transparent px-2 text-sm">
        {#each protos as [value, text] (value)}
          <option {value}>{text()}</option>
        {/each}
      </select>
    </Label>
    <Label class="grid w-32 gap-1">
      {m.port_forwards_external()}
      <Input bind:value={form.externalPort} required maxlength={11} autocomplete="off" spellcheck={false}
        disabled={control.busy} placeholder="8080" class="font-mono" />
    </Label>
    <Label class="grid min-w-40 flex-1 gap-1">
      {m.port_forwards_target_ip()}
      <Input bind:value={form.targetIP} required maxlength={15} autocomplete="off" spellcheck={false}
        disabled={control.busy} placeholder="192.168.1.20" class="font-mono" list="port-forward-clients" />
    </Label>
    <Label class="grid w-32 gap-1">
      {m.port_forwards_target_port()}
      <Input bind:value={form.targetPort} maxlength={11} autocomplete="off" spellcheck={false}
        disabled={control.busy} placeholder={m.port_forwards_target_port_same()} class="font-mono" />
    </Label>
    <Button variant="outline" type="submit" disabled={control.busy}>
      {control.applying ? m.port_forwards_applying() : m.port_forwards_add()}
    </Button>
  </form>
  <datalist id="port-forward-clients">
    {#each lan.data?.clients ?? [] as c (c.mac)}
      {#if c.ip}
        <option value={c.ip}>{clientName(c)}</option>
      {/if}
    {/each}
  </datalist>
  <p class="mt-2 text-sm text-muted-foreground">{m.port_forwards_note()}</p>
</Card>

<ConfirmDialog {confirm} />
