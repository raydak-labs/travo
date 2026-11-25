<script lang="ts">
import Download from '@lucide/svelte/icons/download';
import RefreshCw from '@lucide/svelte/icons/refresh-cw';
import ScrollText from '@lucide/svelte/icons/scroll-text';
import Search from '@lucide/svelte/icons/search';
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import { fetchSystemLog, LOG_LEVELS, type LogLevel, levelKind, shortTime } from '../api/system-log';
import { m } from '../paraglide/messages.js';
import EmptyState from '../ui/EmptyState.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import PageHeader from '../ui/PageHeader.svelte';
import Section from '../ui/Section.svelte';
import SelectField from '../ui/SelectField.svelte';
import Status from '../ui/Status.svelte';

let level = $state<LogLevel | ''>('');
let filter = $state('');
// The loader reads the applied query; typing refetches once the user pauses.
let applied = { lines: 500, level: undefined as LogLevel | undefined, filter: '' };
const log = createLoader((signal) => fetchSystemLog(applied, signal));
let typing: ReturnType<typeof setTimeout> | undefined;
onDestroy(() => clearTimeout(typing));

function apply() {
  clearTimeout(typing);
  applied = { lines: 500, level: level || undefined, filter: filter.trim() };
  void log.retry();
}

function onFilterInput() {
  clearTimeout(typing);
  typing = setTimeout(apply, 400);
}

function download() {
  const entries = log.data?.entries ?? [];
  const text = entries
    .map((e) => [e.time, e.level ?? '', e.program ?? '', e.message].join('\t'))
    .join('\n');
  const url = URL.createObjectURL(new Blob([`${text}\n`], { type: 'text/plain' }));
  const link = document.createElement('a');
  link.href = url;
  link.download = 'router-log.txt';
  link.click();
  URL.revokeObjectURL(url);
}
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_system_logs()} description={m.system_logs_description()}>
    {#snippet actions()}
      <Button variant="outline" size="sm" type="button" disabled={!log.data?.entries.length} onclick={download}><Download aria-hidden="true" />{m.logs_download()}</Button>
    {/snippet}
  </PageHeader>

  <Section title={m.logs_title()} description={m.logs_intro()}>
    <div class="flex flex-wrap items-center gap-2">
      <SelectField bind:value={level} class="w-48" onchange={apply}
        options={[{ value: '', label: m.logs_level_all() }, ...LOG_LEVELS.map((option) => ({ value: option, label: m.logs_level_at_least({ level: option }) }))]} />
      <div class="relative min-w-40 flex-1">
        <Search aria-hidden="true" size={16} class="pointer-events-none absolute top-1/2 left-2.5 -translate-y-1/2 text-muted-foreground" />
        <Input class="pl-8" type="search" bind:value={filter} oninput={onFilterInput} maxlength={128} autocomplete="off" spellcheck={false}
          aria-label={m.logs_search()} placeholder={m.logs_filter_hint()} />
      </div>
      <Button variant="ghost" size="icon" type="button" aria-label={m.logs_refresh()} onclick={apply}><RefreshCw aria-hidden="true" size={16} /></Button>
    </div>

    <LoadState status={log.status} retry={log.retry} stale={log.data !== undefined}>
      {#if log.data}
        {#if log.data.entries.length === 0}
          <EmptyState icon={ScrollText} title={m.logs_empty()} />
        {:else}
          <ol class="max-h-[60vh] overflow-y-auto rounded-md border border-border font-mono text-xs" aria-label={m.logs_title()}>
            {#each log.data.entries as entry, i (i)}
              <li class="border-b border-border last:border-b-0">
                <details class="group">
                  <summary class="flex cursor-pointer list-none flex-wrap items-center gap-x-2 gap-y-0.5 px-3 py-1 sm:flex-nowrap hover:bg-muted/50 [&::-webkit-details-marker]:hidden">
                    <span class="shrink-0 text-muted-foreground tabular-nums">{shortTime(entry.time)}</span>
                    <span class="w-[4.5rem] shrink-0">{#if entry.level}<Status kind={levelKind(entry.level)}>{entry.level}</Status>{/if}</span>
                    {#if entry.program}<span class="w-28 shrink-0 truncate font-semibold">{entry.program}</span>{/if}
                    <span class="min-w-0 truncate group-open:hidden max-sm:basis-full sm:flex-1">{entry.message}</span>
                  </summary>
                  <div class="px-3 pb-2 break-all whitespace-pre-wrap">
                    {entry.message}
                  </div>
                </details>
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
