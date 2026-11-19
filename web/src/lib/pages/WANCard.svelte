<script lang="ts">
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
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
import Card from '../ui/Card.svelte';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
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
const control = createTunnelControl(() => loader.retry(), {
  submit: (_enabled, signal) => setWanSettings({ body: requestFrom(form), signal }),
  refusal: wanRefusal,
  confirmed: m.wan_confirmed,
  rolledBack: m.wan_rolled_back,
  rollbackFailed: m.wan_rollback_failed,
  failed: m.wan_failed,
});
transactionToast(
  'wan',
  () => controlPhase(control, m.wan_applying()),
  (ok) => void control.settle(ok),
);
const confirm = createConfirm();
onDestroy(() => control.stop());

// A settled transaction (kept or rolled back) refills the form from the next load; a refusal
// keeps the user's edits.
let settledID = '';
$effect(() => {
  const t = control.transaction;
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
  if (control.busy) return;
  const ok = await confirm.ask({
    title: m.wan_confirm_title(),
    description: m.wan_confirm(),
    action: m.wan_apply(),
  });
  if (ok) void control.set(true);
}
</script>

<Card title={m.wan_title()}>
  <p class="text-muted-foreground">{m.wan_intro()}</p>
  <LoadState status={loader.status} retry={loader.retry} stale={loader.data !== undefined}>
    {#if loader.data}
      <form class="mt-3 flex flex-col gap-4" onsubmit={save}>
        <fieldset class="flex flex-wrap gap-4" disabled={control.busy}>
          <legend class="sr-only">{m.wan_proto()}</legend>
          {#each protos as [value, label] (value)}
            <label class="flex items-center gap-2">
              <input type="radio" name="wan-proto" {value} bind:group={form.proto} />
              {label()}
            </label>
          {/each}
        </fieldset>
        {#if form.proto === 'static'}
          <div class="flex flex-wrap gap-3">
            <Label class="grid w-48 gap-1">
              {m.wan_ipaddr()}
              <Input bind:value={form.ipaddr} required maxlength={18} placeholder="203.0.113.5/24"
                autocomplete="off" spellcheck={false} disabled={control.busy} />
            </Label>
            <Label class="grid w-40 gap-1">
              {m.wan_gateway()}
              <Input bind:value={form.gateway} required maxlength={15} placeholder="203.0.113.1"
                autocomplete="off" spellcheck={false} disabled={control.busy} />
            </Label>
            <Label class="grid w-64 gap-1">
              {m.wan_dns()}
              <Input bind:value={form.dns} maxlength={80} placeholder="1.1.1.1, 9.9.9.9"
                autocomplete="off" spellcheck={false} disabled={control.busy} />
            </Label>
          </div>
        {:else if form.proto === 'pppoe'}
          <div class="flex flex-wrap gap-3">
            <Label class="grid w-56 gap-1">
              {m.wan_username()}
              <Input bind:value={form.username} required maxlength={128} autocomplete="off"
                spellcheck={false} disabled={control.busy} />
            </Label>
            <Label class="grid w-56 gap-1">
              {m.wan_password()}
              <Input type="password" bind:value={form.password} maxlength={128} autocomplete="new-password"
                required={!loader.data.password_set} disabled={control.busy}
                placeholder={loader.data.password_set ? m.wan_password_keep() : ''} />
            </Label>
          </div>
        {/if}
        <p class="text-sm text-muted-foreground">{m.wan_device({ device: loader.data.device })}</p>
        <div>
          <Button variant="default" type="submit" disabled={control.busy}>
            {control.applying ? m.wan_applying() : m.wan_apply()}
          </Button>
        </div>
      </form>
    {/if}
  </LoadState>
</Card>

<ConfirmDialog {confirm} />
