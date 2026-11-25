<script lang="ts">
import ChevronRight from '@lucide/svelte/icons/chevron-right';
import Button from '$lib/components/ui/button/button.svelte';
import Progress from '$lib/components/ui/progress/progress.svelte';
import { alertMessage } from '../alerts';
import type { Alert, SystemInfoBody } from '../api/gen';
import { formatBytes, formatLoad, formatUptime } from '../format';
import { m } from '../paraglide/messages.js';
import { href } from '../router';
import LoadState from '../ui/LoadState.svelte';
import type { LoadStatus } from '../ui/loader.svelte';
import Section from '../ui/Section.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import Status from '../ui/Status.svelte';

// CPU, memory, storage and uptime in one card. The daemon's own memory_low warning shows here
// (warnings stay on their pages; the banner shows danger and info).
let {
  resources,
  alerts = [],
}: {
  resources: { status: LoadStatus; data?: SystemInfoBody; retry: () => void };
  alerts?: Alert[];
} = $props();

const used = (b: { total_bytes: number; available_bytes: number }) =>
  Math.max(0, b.total_bytes - b.available_bytes);
const percent = (b: { total_bytes: number; available_bytes: number }) =>
  b.total_bytes === 0 ? 0 : Math.round((used(b) / b.total_bytes) * 100);
const memoryWarning = $derived(alerts.find((a) => a.code === 'memory_low'));
</script>

<Section title={m.dashboard_resources()}>
  {#snippet actions()}
    <Button variant="ghost" size="sm" href={href('system')}>{m.dashboard_system_open()}<ChevronRight aria-hidden="true" /></Button>
  {/snippet}
  <LoadState status={resources.status} retry={resources.retry} stale={resources.data !== undefined}>
    {#if resources.data}
      {@const d = resources.data}
      {#if memoryWarning}<Status kind="warn">{alertMessage(memoryWarning)}</Status>{/if}
      <div>
        <SettingsRow label={m.dashboard_cpu_load()}>
          {#snippet control()}<span class="tabular-nums">{formatLoad([d.load[0]])}</span>{/snippet}
        </SettingsRow>
        {#each [[m.system_memory(), d.memory], [m.system_storage(), d.storage]] as const as [label, b] (label)}
          <div data-slot="settings-row" class="flex flex-col gap-2 py-3 [[data-slot=settings-row]+&]:border-t [[data-slot=settings-row]+&]:border-border">
            <div class="flex items-center justify-between gap-4 text-sm">
              <span class="font-medium">{label}</span>
              {#if b.total_bytes === 0}
                <Status kind="neutral">{m.dashboard_unavailable()}</Status>
              {:else}
                <span class="tabular-nums text-muted-foreground">{m.system_used_of({ used: formatBytes(used(b)), total: formatBytes(b.total_bytes) })}</span>
              {/if}
            </div>
            {#if b.total_bytes > 0}<Progress value={percent(b)} aria-label={label} />{/if}
          </div>
        {/each}
        <SettingsRow label={m.system_uptime()}>
          {#snippet control()}<span class="tabular-nums">{formatUptime(d.uptime_seconds)}</span>{/snippet}
        </SettingsRow>
      </div>
    {/if}
  </LoadState>
</Section>
