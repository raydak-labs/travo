<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import * as Dialog from '$lib/components/ui/dialog';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import {
  dayName,
  daysLabel,
  emptyForm,
  fetchSchedule,
  formFrom,
  formProblem,
  lastRun,
  outcomeLabel,
  type RuleForm,
  removeRule,
  runTime,
  saveWithConsent,
  setRuleEnabled,
  upsertRule,
  type WiFiScheduleRule,
  weekOrder,
} from '../api/wifi-schedule';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Status from '../ui/Status.svelte';

const schedule = createLoader(fetchSchedule);
const confirm = createConfirm();
let saving = $state(false);
let open = $state(false);
let form = $state<RuleForm>(emptyForm([]));
const problem = $derived(formProblem(form));

function apName(section: string): string {
  return schedule.data?.aps.find((ap) => ap.section === section)?.ssid || section;
}

// Saves the set `next` builds; the lockout dialog decides the acknowledgement.
async function save(
  next: (acknowledge: boolean) => WiFiScheduleRule[],
  done: string,
  ssid: string,
) {
  if (saving) return false;
  saving = true;
  try {
    const result = await saveWithConsent(next, () =>
      confirm.ask({
        title: m.wifi_schedule_lockout_title(),
        description: m.wifi_schedule_lockout_body({ ssid }),
        action: m.wifi_schedule_save_anyway(),
        tone: 'danger',
      }),
    );
    if ('cancelled' in result) return false;
    if ('error' in result) {
      notify.error(result.error);
      return false;
    }
    notify.success(done);
    schedule.retry();
    return true;
  } finally {
    saving = false;
  }
}

function add() {
  form = emptyForm(schedule.data?.aps ?? []);
  open = true;
}

function edit(rule: WiFiScheduleRule) {
  form = formFrom(rule);
  open = true;
}

async function submit(event: SubmitEvent) {
  event.preventDefault();
  const rules = schedule.data?.rules;
  if (!rules || problem) return;
  const snapshot = { ...form, days: [...form.days] };
  if (
    await save(
      (ack) => upsertRule(rules, snapshot, ack),
      m.wifi_schedule_saved(),
      apName(snapshot.section),
    )
  )
    open = false;
}

function toggleDay(day: number, on: boolean) {
  form.days = on ? [...form.days, day] : form.days.filter((d) => d !== day);
}

function pause(rule: WiFiScheduleRule) {
  const rules = schedule.data?.rules ?? [];
  const enabled = !rule.enabled;
  void save(
    (ack) => setRuleEnabled(rules, rule.id, enabled, ack),
    enabled ? m.wifi_schedule_resumed() : m.wifi_schedule_paused(),
    apName(rule.section),
  );
}

async function remove(rule: WiFiScheduleRule) {
  const ok = await confirm.ask({
    title: m.wifi_schedule_delete_title(),
    description: m.wifi_schedule_delete_body({ ssid: apName(rule.section) }),
    action: m.wifi_schedule_delete(),
    tone: 'danger',
  });
  if (!ok) return;
  const rules = schedule.data?.rules ?? [];
  void save(() => removeRule(rules, rule.id), m.wifi_schedule_deleted(), apName(rule.section));
}
</script>

<LoadState status={schedule.status} retry={schedule.retry} stale={schedule.data !== undefined}>
  {#if schedule.data}
    {@const view = schedule.data}
    <p class="text-sm text-muted-foreground">
      {view.zone ? m.wifi_schedule_zone({ zone: view.zone }) : m.wifi_schedule_local()}
    </p>
    {#if view.cronRunning === false}
      <Status kind="warn">{m.wifi_schedule_cron_stopped()}</Status>
    {/if}
    {#if view.rules.length === 0}
      <p class="mt-3 text-muted-foreground">{m.wifi_schedule_empty()}</p>
    {:else}
      <ul class="mt-3 divide-y divide-border" aria-label={m.wifi_schedule_title()}>
        {#each view.rules as rule (rule.id)}
          {@const run = lastRun(view.runs, rule.id)}
          <li class="flex flex-wrap items-center gap-2 py-2">
            <span class="min-w-0 flex-1">
              <span class="block break-all font-medium">{apName(rule.section)}</span>
              <span class="block text-sm text-muted-foreground">
                {m.wifi_schedule_window({ off: rule.off, on: rule.on, days: daysLabel(rule.days) })}
              </span>
              {#if run}
                <span class="block text-xs text-muted-foreground">
                  {m.wifi_schedule_last({ time: runTime(run.at, view.zone), action: run.action, outcome: outcomeLabel(run.outcome) })}
                </span>
              {/if}
            </span>
            <Status kind={rule.enabled ? 'ok' : 'neutral'}>
              {rule.enabled ? m.wifi_schedule_active() : m.wifi_schedule_paused_badge()}
            </Status>
            <span class="flex gap-2">
              <Button variant="outline" size="sm" type="button" disabled={saving}
                aria-label={m.wifi_schedule_edit_name({ ssid: apName(rule.section) })}
                onclick={() => edit(rule)}>{m.wifi_ap_edit()}</Button>
              <Button variant="outline" size="sm" type="button" disabled={saving} onclick={() => pause(rule)}>
                {rule.enabled ? m.wifi_schedule_pause() : m.wifi_schedule_resume()}
              </Button>
              <Button variant="outline" size="sm" type="button" disabled={saving}
                aria-label={m.wifi_schedule_delete_name({ ssid: apName(rule.section) })}
                onclick={() => remove(rule)}>{m.wifi_schedule_delete()}</Button>
            </span>
          </li>
        {/each}
      </ul>
    {/if}
    <Button variant="outline" size="sm" type="button" class="mt-3 w-fit"
      disabled={saving || view.aps.length === 0 || view.rules.length >= 16} onclick={add}>
      {m.wifi_schedule_add()}
    </Button>
  {/if}
</LoadState>

<Dialog.Root bind:open>
<Dialog.Content portalProps={{ disabled: typeof window === "undefined" }} showCloseButton={false}
 aria-label={m.wifi_schedule_dialog()} class="w-[min(90vw,26rem)] rounded-lg border border-border bg-surface p-5 text-fg">
  <form class="space-y-4" onsubmit={submit}>
    <Dialog.Title level={2} class="text-lg font-semibold">{m.wifi_schedule_dialog()}</Dialog.Title>
    <Label class="grid gap-1">
      {m.wifi_schedule_ap()}
      <select bind:value={form.section} disabled={saving} required
        class="h-9 rounded-md border border-input bg-transparent px-2 text-sm">
        {#each schedule.data?.aps ?? [] as ap (ap.section)}
          <option value={ap.section}>{ap.ssid || ap.section}{ap.guest ? ` (${m.wifi_ap_guest()})` : ''}</option>
        {/each}
      </select>
    </Label>
    <div class="flex gap-3">
      <Label class="grid flex-1 gap-1">
        {m.wifi_schedule_off()}
        <Input type="time" bind:value={form.off} required disabled={saving} />
      </Label>
      <Label class="grid flex-1 gap-1">
        {m.wifi_schedule_on()}
        <Input type="time" bind:value={form.on} required disabled={saving} />
      </Label>
    </div>
    <fieldset class="grid gap-1">
      <legend class="text-sm font-medium">{m.wifi_schedule_days()}</legend>
      <div class="flex flex-wrap gap-3">
        {#each weekOrder as day (day)}
          <label class="flex items-center gap-1 text-sm">
            <input type="checkbox" checked={form.days.includes(day)} disabled={saving}
              onchange={(e) => toggleDay(day, e.currentTarget.checked)} />
            {dayName(day)}
          </label>
        {/each}
      </div>
    </fieldset>
    <p class="text-sm text-muted-foreground">{m.wifi_schedule_hint()}</p>
    {#if problem}
      <p class="text-sm text-danger" role="alert">{problem}</p>
    {/if}
    <div class="flex justify-end gap-2">
      <Button variant="outline" type="button" onclick={() => (open = false)}>{m.wifi_join_cancel()}</Button>
      <Button variant="outline" type="submit" disabled={saving || problem !== undefined}>{m.wifi_ap_save()}</Button>
    </div>
  </form>
</Dialog.Content>
</Dialog.Root>

<ConfirmDialog {confirm} />
