<script lang="ts">
import Download from '@lucide/svelte/icons/download';
import Button from '$lib/components/ui/button/button.svelte';
import '../api/client';
import { downloadSystemBackup, fetchSystemInfo } from '../api/system';
import { formatBytes, formatLoad, formatUptime } from '../format';
import { m } from '../paraglide/messages.js';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import PageHeader from '../ui/PageHeader.svelte';
import Section from '../ui/Section.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import SummaryBand from '../ui/SummaryBand.svelte';
import SummaryTile from '../ui/SummaryTile.svelte';
import SystemSettingsCard from './SystemSettingsCard.svelte';

// The System overview: what this router is, its name and time, and the backup. Restart, the
// theme and Log out live in the header menu; access (tokens, keys, password) has its own page.
const info = createLoader(fetchSystemInfo);
let backupPending = $state(false);

async function downloadBackup() {
  backupPending = true;
  try {
    await downloadSystemBackup();
  } catch {
    notify.error(m.system_backup_failed());
  } finally {
    backupPending = false;
  }
}

const used = (total: number, available: number) =>
  m.system_used_of({ used: formatBytes(total - available), total: formatBytes(total) });
</script>

{#snippet fact(value: string)}
  <LoadState status={info.status} retry={info.retry} stale={info.data !== undefined}>
    <span class="text-base font-semibold">{value}</span>
  </LoadState>
{/snippet}

{#snippet text(value: string, mono = false)}
  <span class="text-sm {mono ? 'font-mono' : ''}">{value}</span>
{/snippet}

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_group_system()} description={m.system_device_description()}>
    {#snippet actions()}
      <Button variant="outline" size="sm" type="button" disabled={backupPending} onclick={downloadBackup}>
        <Download aria-hidden="true" />{backupPending ? m.system_backup_pending() : m.system_backup_download()}
      </Button>
    {/snippet}
  </PageHeader>

  <SummaryBand title={m.system_device()}>
    <SummaryTile label={m.system_model()}>{@render fact(info.data?.model ?? '')}</SummaryTile>
    <SummaryTile label={m.system_openwrt()}>{@render fact(info.data?.openwrt ?? '')}</SummaryTile>
    <SummaryTile label={m.system_uptime()}>{@render fact(info.data ? formatUptime(info.data.uptime_seconds) : '')}</SummaryTile>
  </SummaryBand>

  <SystemSettingsCard />

  <Section title={m.system_details()}>
    <LoadState status={info.status} retry={info.retry} stale={info.data !== undefined}>
      {#if info.data}
        {@const d = info.data}
        <div>
          <SettingsRow label={m.system_openwrt()}>{#snippet control()}{@render text(`${d.openwrt} ${d.revision}`)}{/snippet}</SettingsRow>
          <SettingsRow label={m.system_kernel()}>{#snippet control()}{@render text(d.kernel)}{/snippet}</SettingsRow>
          <SettingsRow label={m.system_load()}>{#snippet control()}{@render text(formatLoad(d.load))}{/snippet}</SettingsRow>
          <SettingsRow label={m.system_memory()}>{#snippet control()}{@render text(used(d.memory.total_bytes, d.memory.available_bytes))}{/snippet}</SettingsRow>
          <SettingsRow label={m.system_storage()}>{#snippet control()}{@render text(used(d.storage.total_bytes, d.storage.available_bytes))}{/snippet}</SettingsRow>
          <SettingsRow label={m.system_app_version()}>{#snippet control()}{@render text(d.app.version)}{/snippet}</SettingsRow>
          <SettingsRow label={m.system_build_time()}>{#snippet control()}{@render text(d.app.build_time || '—')}{/snippet}</SettingsRow>
        </div>
      {/if}
    </LoadState>
  </Section>
</div>
