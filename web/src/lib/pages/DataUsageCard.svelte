<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import {
  clearDataBudget,
  type DataUsageBudget,
  type DataUsageView,
  fetchDataUsage,
  saveDataBudget,
} from '../api/data-usage';
import {
  bytesToGib,
  dailySamples,
  dayFrame,
  dayUplinks,
  gibToBytes,
  recentEvents,
  uplinkTotals,
  usedPercent,
} from '../data-usage';
import { formatBytes } from '../format';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Sparkline from '../ui/Sparkline.svelte';
import Status from '../ui/Status.svelte';
import type { StatusKind } from '../ui/status';

const ALL_UPLINKS = ['wan', 'wwan', 'usbtether'] as const;
const loader = createLoader(fetchDataUsage);
const confirm = createConfirm();
let range = $state<'today' | 'billing'>('billing');
let form = $state({
  enabled: false,
  gib: 10,
  warn: 80,
  startDay: 1,
  uplinks: [...ALL_UPLINKS] as string[],
});
let filled = false;
let busy = $state(false);

// Fill the form once per load; a later reload must not wipe edits.
$effect(() => {
  const b = loader.data?.budget;
  if (!b || filled) return;
  filled = true;
  form = {
    enabled: b.enabled,
    gib: b.limit_bytes > 0 ? bytesToGib(b.limit_bytes) : 10,
    warn: b.warn_percent,
    startDay: b.start_day,
    uplinks: [...b.uplinks],
  };
});

const stateKind: Record<DataUsageView['budget_state'], StatusKind> = {
  off: 'neutral',
  ok: 'ok',
  warning: 'warn',
  exceeded: 'danger',
  unreadable: 'warn',
};

function stateText(v: DataUsageView): string {
  switch (v.budget_state) {
    case 'off':
      return m.usage_budget_off();
    case 'unreadable':
      return m.usage_budget_unreadable();
    default:
      return m.usage_budget_used({
        used: formatBytes(v.budget_used_bytes),
        limit: formatBytes(v.budget.limit_bytes),
        percent: usedPercent(v.budget_used_bytes, v.budget.limit_bytes),
      });
  }
}

function when(at: number): string {
  return new Date(at * 1000).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
}

async function finish(
  action: () => Promise<{ view: DataUsageView } | { error: string }>,
  done: string,
) {
  busy = true;
  const result = await action();
  busy = false;
  if ('error' in result) {
    notify.error(result.error);
    return;
  }
  notify.success(done);
  filled = false;
  void loader.retry();
}

function save(event: SubmitEvent) {
  event.preventDefault();
  if (busy) return;
  const budget: DataUsageBudget = {
    enabled: form.enabled,
    limit_bytes: form.enabled ? gibToBytes(form.gib) : 0,
    warn_percent: form.warn,
    start_day: form.startDay,
    uplinks: ALL_UPLINKS.filter((u) => form.uplinks.includes(u)),
  };
  void finish(() => saveDataBudget(budget), m.usage_budget_saved());
}

async function reset() {
  const ok = await confirm.ask({
    title: m.usage_reset_title(),
    description: m.usage_reset_body(),
    action: m.usage_reset(),
    tone: 'danger',
  });
  if (ok && !busy) void finish(clearDataBudget, m.usage_reset_done());
}
</script>

<Card title={m.usage_title()}>
  <p class="text-muted-foreground">{m.usage_intro()}</p>
  <LoadState status={loader.status} retry={loader.retry} stale={loader.data !== undefined}>
    {#if loader.data}
      {@const v = loader.data}
      {@const period = range === 'today' ? v.today : v.billing_period}
      {@const frame = dayFrame(v.days)}
      <div class="mt-3 flex flex-col gap-4">
        <div class="flex flex-wrap items-center gap-2">
          <Status kind={stateKind[v.budget_state]}>{stateText(v)}</Status>
          {#if v.budget_state === 'unreadable'}
            <Button variant="outline" size="sm" type="button" disabled={busy} onclick={reset}>{m.usage_reset()}</Button>
          {/if}
          {#if v.persistence === 'degraded'}
            <Status kind="warn">{m.history_not_saved()}</Status>
          {/if}
        </div>
        <div class="flex gap-2" role="group" aria-label={m.usage_range_label()}>
          {#each [['today', m.usage_today()], ['billing', m.usage_billing()]] as const as [key, text] (key)}
            <Button size="sm" type="button" variant={range === key ? 'default' : 'outline'}
              aria-pressed={range === key} onclick={() => (range = key)}>{text}</Button>
          {/each}
        </div>
        <p class="text-sm text-muted-foreground">{m.usage_period({ start: period.start, end: period.end })}</p>
        {#if period.uplinks.length === 0}
          <p class="text-muted-foreground">{m.usage_none()}</p>
        {:else}
          <dl class="grid gap-1 text-sm">
            {#each uplinkTotals(period.uplinks) as row (row.name)}
              <div class="flex justify-between gap-2">
                <dt>{row.name}</dt>
                <dd class="tabular-nums">
                  {m.usage_row({ total: formatBytes(row.total), rx: formatBytes(row.rx), tx: formatBytes(row.tx) })}
                </dd>
              </div>
            {/each}
          </dl>
        {/if}
        {#if frame}
          <dl class="grid gap-3">
            {#each dayUplinks(v.days) as name (name)}
              <div>
                <dt class="text-sm">{m.usage_daily({ uplink: name })}</dt>
                <dd><Sparkline samples={dailySamples(v.days, name)} {frame} label={m.usage_daily({ uplink: name })} /></dd>
              </div>
            {/each}
          </dl>
        {/if}

        <form class="flex flex-col gap-3" onsubmit={save}>
          <h3 class="font-medium">{m.usage_budget()}</h3>
          <label class="flex items-center gap-2">
            <input type="checkbox" bind:checked={form.enabled} disabled={busy} />
            {m.usage_budget_enabled()}
          </label>
          <div class="flex flex-wrap gap-3">
            <Label class="grid w-32 gap-1">
              {m.usage_limit_gib()}
              <Input type="number" min={0.01} step={0.01} bind:value={form.gib} required={form.enabled} disabled={busy || !form.enabled} />
            </Label>
            <Label class="grid w-32 gap-1">
              {m.usage_warn_percent()}
              <Input type="number" min={1} max={100} bind:value={form.warn} required disabled={busy} />
            </Label>
            <Label class="grid w-32 gap-1">
              {m.usage_start_day()}
              <Input type="number" min={1} max={28} bind:value={form.startDay} required disabled={busy} />
            </Label>
          </div>
          <fieldset class="flex flex-wrap gap-3 text-sm">
            <legend class="mb-1">{m.usage_counted()}</legend>
            {#each ALL_UPLINKS as u (u)}
              <label class="flex items-center gap-1">
                <input type="checkbox" value={u} bind:group={form.uplinks} disabled={busy} />{u}
              </label>
            {/each}
          </fieldset>
          <p class="text-sm text-muted-foreground">{m.usage_budget_note()}</p>
          <div><Button type="submit" disabled={busy || form.uplinks.length === 0}>{m.usage_save()}</Button></div>
        </form>

        <div class="flex flex-col gap-2">
          <h3 class="font-medium">{m.usage_uptime_title({ days: Math.round(v.retention_hours / 24) })}</h3>
          {#each v.uptime.availability as a (a.name)}
            <p class="text-sm">{m.usage_availability({ uplink: a.name, percent: usedPercent(a.up_samples, a.samples) })}</p>
          {/each}
          {#if v.uptime.events.length === 0}
            <p class="text-sm text-muted-foreground">{m.usage_no_events()}</p>
          {:else}
            <ul class="grid gap-1 text-sm">
              {#each recentEvents(v.uptime.events) as e, i (i)}
                <li class="flex justify-between gap-2">
                  <span>
                    {e.kind === 'boot' ? m.usage_event_boot() : e.kind === 'up' ? m.usage_event_up({ uplink: e.uplink ?? '' }) : m.usage_event_down({ uplink: e.uplink ?? '' })}
                  </span>
                  <span class="text-muted-foreground tabular-nums">{when(e.at)}</span>
                </li>
              {/each}
            </ul>
          {/if}
          <p class="text-sm text-muted-foreground">{m.usage_uptime_note()}</p>
        </div>
      </div>
    {/if}
  </LoadState>
</Card>
<ConfirmDialog {confirm} />
