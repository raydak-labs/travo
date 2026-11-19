<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import type { LedSchedule } from '../api/gen';
import { fetchLEDs, saveLEDSchedule, scheduleProblem, setLEDsOff } from '../api/leds';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Status from '../ui/Status.svelte';

const loader = createLoader(fetchLEDs);
let busy = $state(false);
let form = $state<LedSchedule>({ enabled: false, off_at: '23:00', on_at: '07:00' });
let filled = false;
const problem = $derived(scheduleProblem(form));

// Fill the schedule once from the first load; a later reload must not wipe edits.
$effect(() => {
  const data = loader.data;
  if (!data || filled) return;
  filled = true;
  form = { ...data.schedule };
});

async function run(action: () => ReturnType<typeof setLEDsOff>, done: string) {
  if (busy) return;
  busy = true;
  const result = await action();
  busy = false;
  if ('error' in result) {
    notify.error(result.error);
    return;
  }
  notify.success(done);
  void loader.retry();
}

function toggle(off: boolean) {
  void run(() => setLEDsOff(off), off ? m.leds_turned_off() : m.leds_turned_default());
}

function save(event: SubmitEvent) {
  event.preventDefault();
  if (problem) return;
  const snapshot = { ...form };
  filled = false;
  void run(() => saveLEDSchedule(snapshot), m.leds_schedule_saved());
}
</script>

<Card title={m.leds_title()}>
  <LoadState status={loader.status} retry={loader.retry} stale={loader.data !== undefined}>
    {#if loader.data}
      {@const view = loader.data}
      {#if view.leds.length === 0}
        <p class="text-muted-foreground">{m.leds_none()}</p>
      {:else}
        <ul class="divide-y divide-border" aria-label={m.leds_title()}>
          {#each view.leds as led (led.name)}
            <li class="flex items-center gap-2 py-2">
              <span class="min-w-0 flex-1 break-all font-mono text-sm">{led.name}</span>
              <Status kind={led.on ? 'ok' : 'neutral'}>{led.on ? m.leds_lit() : m.leds_dark()}</Status>
            </li>
          {/each}
        </ul>
        <div class="mt-3 flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm" type="button" disabled={busy} onclick={() => toggle(!view.off)}>
            {view.off ? m.leds_default() : m.leds_off()}
          </Button>
          {#if view.off}<Status kind="info">{m.leds_held_off()}</Status>{/if}
        </div>
      {/if}
      <form class="mt-4 flex flex-col gap-3" onsubmit={save}>
        <label class="flex items-center gap-2">
          <input type="checkbox" bind:checked={form.enabled} disabled={busy} />
          {m.leds_schedule()}
        </label>
        <div class="flex gap-3">
          <Label class="grid flex-1 gap-1">
            {m.leds_schedule_off()}
            <Input type="time" bind:value={form.off_at} required disabled={busy} />
          </Label>
          <Label class="grid flex-1 gap-1">
            {m.leds_schedule_on()}
            <Input type="time" bind:value={form.on_at} required disabled={busy} />
          </Label>
        </div>
        <span class="text-sm text-muted-foreground">{m.leds_schedule_hint()}</span>
        {#if problem}<Status kind="warn">{problem}</Status>{/if}
        <div>
          <Button variant="default" type="submit" disabled={busy || problem !== undefined}>{m.leds_schedule_save()}</Button>
        </div>
      </form>
    {/if}
  </LoadState>
</Card>
