<script lang="ts">
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import { type DhcpView, fetchDHCP, saveDHCP, turnsOff, validLeaseTime } from '../api/dhcp';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';

const loader = createLoader(fetchDHCP);
const confirm = createConfirm();
let enabled = $state(true);
let start = $state(100);
let limit = $state(150);
let leasetime = $state('');
let filled = false;
let busy = $state(false);
let failure = $state<string>();
// The last state the daemon reported: a successful save's answer until the next load, so a
// failed refresh never compares a turn-off against stale data.
let saved = $state<DhcpView>();
const baseline = $derived(saved ?? loader.data);

// Fill the form once from the first successful load; a later reload must not wipe edits.
$effect(() => {
  const data = loader.data;
  if (!data || filled) return;
  filled = true;
  saved = undefined;
  enabled = data.enabled;
  start = data.start;
  limit = data.limit;
  leasetime = data.leasetime;
});

async function save(event: SubmitEvent) {
  event.preventDefault();
  const current = baseline;
  if (!current || busy) return;
  if (!validLeaseTime(leasetime)) {
    failure = m.dhcp_err_leasetime();
    return;
  }
  failure = undefined;
  const next = { enabled, start: Number(start), limit: Number(limit), leasetime: leasetime.trim() };
  const off = turnsOff(current, next);
  const ok = await confirm.ask(
    off
      ? {
          title: m.dhcp_confirm_off_title(),
          description: current.callerLease ? m.dhcp_confirm_off_caller() : m.dhcp_confirm_off(),
          action: m.dhcp_turn_off(),
          tone: 'danger',
        }
      : { title: m.dhcp_confirm_title(), description: m.dhcp_confirm(), action: m.dhcp_save() },
  );
  if (!ok) return;
  busy = true;
  const result = await saveDHCP(next);
  busy = false;
  if ('error' in result) {
    notify.error(result.error);
    return;
  }
  saved = result.view;
  filled = false;
  notify.success(m.dhcp_saved());
  void loader.retry();
}
</script>

<Card title={m.dhcp_title()}>
  <p class="text-muted-foreground">{m.dhcp_intro()}</p>
  <LoadState status={loader.status} retry={loader.retry} stale={loader.data !== undefined}>
    {#if loader.data}
      <form class="mt-3 flex flex-col gap-4" onsubmit={save}>
        <label class="flex items-center gap-2">
          <input type="checkbox" bind:checked={enabled} disabled={busy} />
          {m.dhcp_enabled()}
        </label>
        <div class="flex flex-wrap gap-3">
          <Label class="grid w-36 gap-1">
            {m.dhcp_start()}
            <Input type="number" bind:value={start} min={1} max={254} required disabled={busy} />
          </Label>
          <Label class="grid w-36 gap-1">
            {m.dhcp_limit()}
            <Input type="number" bind:value={limit} min={1} max={254} required disabled={busy} />
          </Label>
          <Label class="grid w-40 gap-1">
            {m.dhcp_leasetime()}
            <Input bind:value={leasetime} required maxlength={16} autocomplete="off" spellcheck={false}
              disabled={busy} aria-invalid={failure !== undefined} />
          </Label>
        </div>
        <span class="text-sm text-muted-foreground">{m.dhcp_leasetime_hint()}</span>
        {#if loader.data.subnet && loader.data.first && loader.data.last}
          <p class="text-sm">{m.dhcp_pool({ first: loader.data.first, last: loader.data.last, subnet: loader.data.subnet })}</p>
        {/if}
        {#if !loader.data.enabled}
          <Alert class="status status-warn self-start">{m.dhcp_off()}</Alert>
        {/if}
        {#if failure}
          <Alert class="status status-danger self-start" role="alert">{failure}</Alert>
        {/if}
        <div>
          <Button variant="default" type="submit" disabled={busy}>{busy ? m.dhcp_saving() : m.dhcp_save()}</Button>
        </div>
      </form>
    {/if}
  </LoadState>
</Card>

<ConfirmDialog {confirm} />
