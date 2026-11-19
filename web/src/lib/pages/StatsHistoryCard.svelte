<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import { clearStatsHistory, fetchStatsHistory, type HistoryRange } from '../api/stats-history';
import { formatBytes, formatUptime } from '../format';
import { m } from '../paraglide/messages.js';
import { historyView, type Sample, summarize } from '../stats-history';
import Card from '../ui/Card.svelte';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Sparkline from '../ui/Sparkline.svelte';
import Status from '../ui/Status.svelte';

const ranges: { hours: HistoryRange; label: () => string }[] = [
  { hours: 24, label: m.history_range_day },
  { hours: 168, label: m.history_range_week },
];

let hours = $state<HistoryRange>(24);
// One-off load per range: points are 10 min buckets, a poll would add nothing.
const history = createLoader((signal) => fetchStatsHistory(hours, signal));
const confirm = createConfirm();
let resetting = $state(false);

const view = $derived(history.data ? historyView(history.data.points) : undefined);
// The router clock frames the window, so a skewed browser clock cannot shift the line.
const frame = $derived.by(() => {
  const end = history.data?.points.at(-1)?.at ?? 0;
  return { start: end - hours * 3600, end, intervalSeconds: history.data?.interval_seconds ?? 600 };
});
const minutes = $derived(Math.round((history.data?.interval_seconds ?? 600) / 60));

function pick(next: HistoryRange) {
  if (next === hours) return;
  hours = next;
  history.retry();
}

function label(title: string, samples: readonly Sample[], format: (n: number) => string) {
  const s = summarize(samples);
  if (!s) return `${title}: ${m.history_no_values()}`;
  return m.history_summary({
    title,
    min: format(s.min),
    max: format(s.max),
    last: format(s.last),
  });
}

async function reset() {
  const ok = await confirm.ask({
    title: m.history_reset_title(),
    description: m.history_reset_body(),
    action: m.history_reset(),
    tone: 'danger',
  });
  if (!ok || resetting) return;
  resetting = true;
  try {
    if (await clearStatsHistory()) {
      notify.success(m.history_reset_done());
      history.retry();
    } else {
      notify.error(m.history_reset_failed());
    }
  } finally {
    resetting = false;
  }
}
</script>

<Card title={m.history_title()}>
  <div class="flex flex-col gap-4">
    <div class="flex gap-2" role="group" aria-label={m.history_range_label()}>
      {#each ranges as range (range.hours)}
        <Button
          size="sm"
          type="button"
          variant={hours === range.hours ? 'default' : 'outline'}
          aria-pressed={hours === range.hours}
          onclick={() => pick(range.hours)}
        >
          {range.label()}
        </Button>
      {/each}
    </div>
    <LoadState status={history.status} retry={history.retry} stale={history.data !== undefined}>
      {#if history.data && view}
        {#if history.data.persistence === 'degraded'}
          <div class="flex flex-wrap items-center gap-2">
            <Status kind="warn">{m.history_not_saved()}</Status>
            <span class="text-sm text-muted-foreground">{m.history_degraded()}</span>
            <Button variant="outline" size="sm" type="button" disabled={resetting} onclick={reset}>
              {m.history_reset()}
            </Button>
          </div>
        {/if}
        {#if history.data.points.length === 0}
          <p class="text-muted-foreground">{m.history_empty({ minutes })}</p>
        {:else}
          <dl class="grid gap-3">
            {#each view.traffic as uplink (uplink.name)}
              {@const title = m.history_traffic({ uplink: uplink.name })}
              <div>
                <dt class="flex justify-between gap-2 text-sm">
                  <span>{title}</span>
                  <span class="text-muted-foreground tabular-nums">
                    {m.history_traffic_total({ bytes: formatBytes(uplink.total) })}
                  </span>
                </dt>
                <dd>
                  <Sparkline
                    samples={uplink.samples}
                    {frame}
                    label={label(m.history_traffic_per({ title, minutes }), uplink.samples, formatBytes)}
                  />
                </dd>
              </div>
            {/each}
            <div>
              <dt class="text-sm">{m.history_clients()}</dt>
              <dd>
                <Sparkline
                  samples={view.clients}
                  {frame}
                  label={label(m.history_clients(), view.clients, String)}
                />
              </dd>
            </div>
            <div>
              <dt class="flex justify-between gap-2 text-sm">
                <span>{m.history_uptime()}</span>
                <span class="text-muted-foreground">{m.history_reboots({ count: view.reboots })}</span>
              </dt>
              <dd>
                <Sparkline
                  samples={view.uptime}
                  {frame}
                  label={label(m.history_uptime(), view.uptime, formatUptime)}
                />
              </dd>
            </div>
          </dl>
        {/if}
      {/if}
    </LoadState>
  </div>
</Card>
<ConfirmDialog {confirm} />
