<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Switch from '$lib/components/ui/switch/switch.svelte';
import { type DhcpView, fetchDHCP, saveDHCP, turnsOff, validLeaseTime } from '../api/dhcp';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Section from '../ui/Section.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';

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
const formId = $props.id();
</script>

<Section title={m.dhcp_title()} description={m.dhcp_intro()}>
  <LoadState status={loader.status} retry={loader.retry} stale={loader.data !== undefined}>
    {#if loader.data}
      {#if !loader.data.enabled}
        <p class="text-sm text-warn" role="status">{m.dhcp_off()}</p>
      {/if}
      <form id={formId} onsubmit={save}>
        <SettingsRow label={m.dhcp_enabled()} for="{formId}-on"
          description={loader.data.subnet && loader.data.first && loader.data.last ? m.dhcp_pool({ first: loader.data.first, last: loader.data.last, subnet: loader.data.subnet }) : undefined}>
          {#snippet control()}<Switch id="{formId}-on" bind:checked={enabled} disabled={busy} />{/snippet}
        </SettingsRow>
        <SettingsRow label={m.dhcp_start()} for="{formId}-start">
          {#snippet control()}<Input id="{formId}-start" class="w-28" type="number" bind:value={start} min={1} max={254} required disabled={busy} />{/snippet}
        </SettingsRow>
        <SettingsRow label={m.dhcp_limit()} for="{formId}-limit">
          {#snippet control()}<Input id="{formId}-limit" class="w-28" type="number" bind:value={limit} min={1} max={254} required disabled={busy} />{/snippet}
        </SettingsRow>
        <SettingsRow label={m.dhcp_leasetime()} for="{formId}-lease" description={m.dhcp_leasetime_hint()}>
          {#snippet control()}<Input id="{formId}-lease" class="w-28" bind:value={leasetime} required maxlength={16} autocomplete="off" spellcheck={false} disabled={busy} aria-invalid={failure !== undefined} />{/snippet}
        </SettingsRow>
        {#if failure}<p class="text-sm text-danger" role="alert">{failure}</p>{/if}
      </form>
    {/if}
  </LoadState>
  {#snippet footer()}
    <Button variant="default" type="submit" form={formId} disabled={busy || !loader.data}>{busy ? m.dhcp_saving() : m.dhcp_save()}</Button>
  {/snippet}
</Section>

<ConfirmDialog {confirm} />
