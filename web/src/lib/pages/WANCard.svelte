<script lang="ts">
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';

import { setWanSettings } from '../api/gen';
import {
  fetchWAN,
  formFrom,
  requestFrom,
  type WanForm,
  type WanProto,
  wanRefusal,
} from '../api/wan';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import Section from '../ui/Section.svelte';
import SelectField from '../ui/SelectField.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import { transactionToast } from '../ui/transaction-toast.svelte';
import { controlPhase, createTunnelControl } from '../vpn-tunnel.svelte';

const loader = createLoader(fetchWAN);
let form = $state<WanForm>({
  proto: 'dhcp',
  ipaddr: '',
  gateway: '',
  dns: '',
  username: '',
  password: '',
});
let filled = false;
const txn = createTunnelControl(() => loader.retry(), {
  submit: (_enabled, signal) => setWanSettings({ body: requestFrom(form), signal }),
  refusal: wanRefusal,
  confirmed: m.wan_confirmed,
  rolledBack: m.wan_rolled_back,
  rollbackFailed: m.wan_rollback_failed,
  failed: m.wan_failed,
});
transactionToast(
  'wan',
  () => controlPhase(txn, m.wan_applying()),
  (ok) => void txn.settle(ok),
);
const confirm = createConfirm();
onDestroy(() => txn.stop());

// A settled transaction (kept or rolled back) refills the form from the next load; a refusal
// keeps the user's edits.
let settledID = '';
$effect(() => {
  const t = txn.transaction;
  if (!t || t.state === 'pending' || t.state === 'restoring' || t.id === settledID) return;
  settledID = t.id;
  filled = false;
});

// Fill the form once per load cycle; a later reload must not wipe edits.
$effect(() => {
  const data = loader.data;
  if (!data || filled) return;
  filled = true;
  form = formFrom(data);
});

const protos: [WanProto, () => string][] = [
  ['dhcp', m.wan_proto_dhcp],
  ['static', m.wan_proto_static],
  ['pppoe', m.wan_proto_pppoe],
];

async function save(event: SubmitEvent) {
  event.preventDefault();
  if (txn.busy) return;
  const ok = await confirm.ask({
    title: m.wan_confirm_title(),
    description: m.wan_confirm(),
    action: m.wan_apply(),
  });
  if (ok) void txn.set(true);
}
const formId = $props.id();
</script>

{#snippet field(id: string, label: string, input: import('svelte').Snippet<[string]>)}
  <SettingsRow {label} for={id}>
    {#snippet control()}{@render input(id)}{/snippet}
  </SettingsRow>
{/snippet}

<Section title={m.wan_title()} description={m.wan_intro()}>
  <LoadState status={loader.status} retry={loader.retry} stale={loader.data !== undefined}>
    {#if loader.data}
      <form id={formId} onsubmit={save}>
        <SettingsRow label={m.wan_proto()} description={m.wan_device({ device: loader.data.device })}>
          {#snippet control()}
            <SelectField bind:value={form.proto} class="w-64" disabled={txn.busy}
              options={protos.map(([value, label]) => ({ value, label: label() }))} />
          {/snippet}
        </SettingsRow>
        {#if form.proto === 'static'}
          {#snippet ip(id: string)}<Input {id} class="w-64" bind:value={form.ipaddr} required maxlength={18} placeholder="203.0.113.5/24" autocomplete="off" spellcheck={false} disabled={txn.busy} />{/snippet}
          {#snippet gw(id: string)}<Input {id} class="w-64" bind:value={form.gateway} required maxlength={15} placeholder="203.0.113.1" autocomplete="off" spellcheck={false} disabled={txn.busy} />{/snippet}
          {#snippet dns(id: string)}<Input {id} class="w-64" bind:value={form.dns} maxlength={80} placeholder="1.1.1.1, 9.9.9.9" autocomplete="off" spellcheck={false} disabled={txn.busy} />{/snippet}
          {@render field(`${formId}-ip`, m.wan_ipaddr(), ip)}
          {@render field(`${formId}-gw`, m.wan_gateway(), gw)}
          {@render field(`${formId}-dns`, m.wan_dns(), dns)}
        {:else if form.proto === 'pppoe'}
          {#snippet user(id: string)}<Input {id} class="w-64" bind:value={form.username} required maxlength={128} autocomplete="off" spellcheck={false} disabled={txn.busy} />{/snippet}
          {#snippet pass(id: string)}<Input {id} class="w-64" type="password" bind:value={form.password} maxlength={128} autocomplete="new-password" required={!loader.data?.password_set} disabled={txn.busy} placeholder={loader.data?.password_set ? m.wan_password_keep() : ''} />{/snippet}
          {@render field(`${formId}-user`, m.wan_username(), user)}
          {@render field(`${formId}-pass`, m.wan_password(), pass)}
        {/if}
      </form>
    {/if}
  </LoadState>
  {#snippet footer()}
    <Button variant="default" type="submit" form={formId} disabled={txn.busy || !loader.data}>
      {txn.applying ? m.wan_applying() : m.wan_apply()}
    </Button>
  {/snippet}
</Section>

<ConfirmDialog {confirm} />
