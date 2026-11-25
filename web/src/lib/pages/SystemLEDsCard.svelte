<script lang="ts">
import Lightbulb from '@lucide/svelte/icons/lightbulb';
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Switch from '$lib/components/ui/switch/switch.svelte';
import type { LedSchedule } from '../api/gen';
import { fetchLEDs, ledLabel, saveLEDSchedule, scheduleProblem, setLEDsOff } from '../api/leds';
import { m } from '../paraglide/messages.js';
import EmptyState from '../ui/EmptyState.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Section from '../ui/Section.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import Status from '../ui/Status.svelte';

const loader = createLoader(fetchLEDs);
let busy = $state(false);
// Start from data that is already loaded (no flash of the defaults); else the effect fills it.
let form = $state<LedSchedule>(
  loader.data ? { ...loader.data.schedule } : { enabled: false, off_at: '23:00', on_at: '07:00' },
);
let filled = loader.data !== undefined;
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
const formId = $props.id();
</script>

<Section title={m.leds_title()} description={loader.data?.leds.length ? (loader.data.off ? m.leds_held_off() : m.leds_all_hint()) : undefined}>
  {#snippet actions()}
    {#if loader.data?.leds.length}
      {@const view = loader.data}
      <Switch id="leds-all" aria-label={m.leds_all()} bind:checked={() => !view.off, (on) => toggle(!on)} disabled={busy} />
    {/if}
  {/snippet}
  <LoadState status={loader.status} retry={loader.retry} stale={loader.data !== undefined}>
    {#if loader.data}
      {@const view = loader.data}
      {#if view.leds.length === 0}
        <EmptyState icon={Lightbulb} title={m.leds_none()} />
      {:else}
        <div aria-label={m.leds_title()} role="group" class="border-t border-border">

          {#each view.leds as led (led.name)}
            <SettingsRow label={ledLabel(led.name)} description={led.name}>
              {#snippet control()}
                <Status kind={led.on ? 'ok' : 'neutral'}>{led.on ? m.leds_lit() : m.leds_dark()}</Status>
              {/snippet}
            </SettingsRow>
          {/each}
        </div>
      {/if}

    {/if}
  </LoadState>
</Section>

<Section title={m.leds_schedule()} description={m.leds_schedule_hint()}>
  {#snippet actions()}
    {#if loader.data}
      <Switch id="leds-night" form={formId} aria-label={m.leds_schedule()} bind:checked={form.enabled} disabled={busy} />
    {/if}
  {/snippet}
  {#if loader.data && form.enabled}
    <form id={formId} class="border-t border-border" onsubmit={save}>
      <SettingsRow label={m.leds_schedule_window()} for="leds-off-at">
        {#snippet control()}
          <Input id="leds-off-at" type="time" class="w-28" aria-label={m.leds_schedule_off()} bind:value={form.off_at} required disabled={busy} />
          <span class="text-muted-foreground">–</span>
          <Input type="time" class="w-28" aria-label={m.leds_schedule_on()} bind:value={form.on_at} required disabled={busy} />
        {/snippet}
      </SettingsRow>
      {#if problem}<Status kind="warn">{problem}</Status>{/if}
    </form>
  {:else if loader.data}
    <form id={formId} onsubmit={save}></form>
  {/if}
  {#snippet footer()}
    {#if loader.data && (form.enabled || loader.data.schedule.enabled)}
      <Button variant="default" type="submit" form={formId} disabled={busy || problem !== undefined}>{m.leds_schedule_save()}</Button>
    {/if}
  {/snippet}
</Section>
