<script lang="ts">
import ScrollText from '@lucide/svelte/icons/scroll-text';
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import { fetchSystemLog, LOG_LEVELS, type LogLevel, levelKind, shortTime } from '../api/system-log';
import { m } from '../paraglide/messages.js';
import { href } from '../router';
import EmptyState from '../ui/EmptyState.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import PageHeader from '../ui/PageHeader.svelte';
import Section from '../ui/Section.svelte';
import SelectField from '../ui/SelectField.svelte';
import Status from '../ui/Status.svelte';

let level = $state<LogLevel | ''>('');
let filter = $state('');
// The loader reads the applied query, so typing does not refetch on every key.
let applied = { lines: 200, level: undefined as LogLevel | undefined, filter: '' };
const log = createLoader((signal) => fetchSystemLog(applied, signal));

function apply(event?: SubmitEvent) {
  event?.preventDefault();
  applied = { lines: 200, level: level || undefined, filter };
  void log.retry();
}
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_system_logs()} description={m.system_logs_description()} />

  <Section title={m.logs_title()} description={m.logs_intro()}>
    <form class="flex flex-wrap items-end gap-3" onsubmit={apply}>
      <Label class="grid gap-1">
        {m.logs_level()}
        <SelectField bind:value={level} class="w-44"
          options={[{ value: '', label: m.logs_level_all() }, ...LOG_LEVELS.map((option) => ({ value: option, label: m.logs_level_at_least({ level: option }) }))]} />
      </Label>
      <Label class="grid min-w-40 flex-1 gap-1">
        {m.logs_filter()}
        <Input bind:value={filter} maxlength={128} autocomplete="off" spellcheck={false}
          placeholder={m.logs_filter_hint()} />
      </Label>
      <Button variant="outline" type="submit">{m.logs_refresh()}</Button>
    </form>

    <LoadState status={log.status} retry={log.retry} stale={log.data !== undefined}>
      {#if log.data}
        {#if log.data.entries.length === 0}
          <EmptyState icon={ScrollText} title={m.logs_empty()} />
        {:else}
          <ol class="divide-y divide-border font-mono text-xs" aria-label={m.logs_title()}>
            {#each log.data.entries as entry, i (i)}
              <li class="flex flex-wrap items-baseline gap-x-2 py-1">
                <span class="text-muted-foreground">{shortTime(entry.time)}</span>
                {#if entry.level}<Status kind={levelKind(entry.level)}>{entry.level}</Status>{/if}
                {#if entry.program}<span class="font-semibold">{entry.program}</span>{/if}
                <span class="w-full break-all whitespace-pre-wrap">{entry.message}</span>
              </li>
            {/each}
          </ol>
          {#if log.data.truncated}
            <p class="text-sm text-muted-foreground">{m.logs_truncated({ count: log.data.entries.length })}</p>
          {/if}
        {/if}
      {/if}
    </LoadState>
  </Section>
</div>
