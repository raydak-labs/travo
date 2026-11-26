<script lang="ts">
import ExternalLink from '@lucide/svelte/icons/external-link';
import { onDestroy } from 'svelte';
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import Switch from '$lib/components/ui/switch/switch.svelte';
import { type AdGuardAction, adguardUrl, changeAdGuard, fetchAdGuard } from '../api/adguard';
import { m } from '../paraglide/messages.js';
import CardGrid from '../ui/CardGrid.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import PageHeader from '../ui/PageHeader.svelte';
import Section from '../ui/Section.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import Status from '../ui/Status.svelte';
import AdGuardTopologyCard from './AdGuardTopologyCard.svelte';
import DoHCard from './DoHCard.svelte';

const request = new AbortController();
const adguard = createLoader(fetchAdGuard, 10000);
let pending = $state<AdGuardAction>();
onDestroy(() => {
  adguard.stop();
  request.abort();
});

async function change(action: AdGuardAction, enabled: boolean) {
  if (pending) return;
  pending = action;
  notify.progress(action === 'install' ? m.adguard_installing() : m.adguard_applying(), 'adguard');
  const result = await changeAdGuard(action, enabled, request.signal);
  if (request.signal.aborted) {
    notify.dismiss('adguard');
    return;
  }
  pending = undefined;
  if ('error' in result) notify.error(result.error, 'adguard');
  else notify.success(m.adguard_done(), 'adguard');
  await adguard.retry();
}

const proxy = adguardUrl(window.location);
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.adguard_title()} description={m.adguard_page_description()}>
    {#snippet actions()}
      {#if adguard.data?.running && adguard.data.loopback}
        <Button variant="outline" size="sm" href={proxy} target="_blank" rel="noopener">{m.adguard_open()}<ExternalLink aria-hidden="true" /></Button>
      {/if}
      {#if adguard.data?.installed}
        {@const st = adguard.data}
        {#if st.busy}<Status kind="info">{m.adguard_busy()}</Status>{/if}
        {#if !st.running}<Status kind="warn">{m.adguard_state_stopped()}</Status>{/if}
        <Switch
          id="adguard-running"
          aria-label={m.adguard_running_label()}
          bind:checked={() => st.running, (on) => void change('service', on)}
          disabled={pending !== undefined || st.busy || (st.topology === 'expert' && st.running)}
        />
      {/if}
    {/snippet}
  </PageHeader>
  <CardGrid cols={3}>
  <Section
    title={m.adguard_dns_title()}
    description={adguard.data?.installed
      ? [m.adguard_version({ version: adguard.data.version }), adguard.data.config_error ? m.adguard_err_config() : '', adguard.data.migration_failed ? m.adguard_migration_failed() : ''].filter(Boolean).join(' · ')
      : undefined}
  >
    <LoadState status={adguard.status} retry={adguard.retry} stale={adguard.data !== undefined}>
      {#if adguard.data}
        {@const st = adguard.data}
        {#if st.fail_open_since}
          <Alert class="text-danger" role="alert">
            <p class="font-semibold">{m.adguard_fail_open_title()}</p>
            <p>{m.adguard_fail_open_body()}</p>
            <Button variant="outline" size="sm" type="button" class="mt-2" disabled={pending !== undefined} onclick={() => change('dns', true)}>{m.adguard_reenable()}</Button>
          </Alert>
        {/if}
        {#if !st.installed}
          <p class="text-sm text-muted-foreground">{m.adguard_absent_body()}</p>
          <Button variant="outline" size="sm" type="button" class="self-start" disabled={pending !== undefined || st.busy} onclick={() => change('install', true)}>{m.adguard_install()}</Button>
        {:else}
          <div>

            {#if st.topology !== 'expert'}
              <SettingsRow
                label={m.adguard_default_dns_label()}
                for="adguard-default-dns"
                description={st.running ? m.adguard_default_dns_help() : m.adguard_default_dns_needs_running()}
              >
                {#snippet control()}
                  <Switch
                    id="adguard-default-dns"
                    bind:checked={() => st.default_dns, (on) => void change('dns', on)}
                    disabled={pending !== undefined || st.busy || !st.running}
                  />
                {/snippet}
              </SettingsRow>
            {/if}
          </div>
        {/if}
      {/if}
    </LoadState>
  </Section>
  {#if adguard.data?.running && adguard.data.topology !== 'expert'}
    <DoHCard />
  {/if}
  {#if adguard.data && (adguard.data.running || adguard.data.topology === 'expert')}
    <AdGuardTopologyCard {...{ expert: adguard.data.topology === 'expert', hosts: adguard.data.topology_hosts ?? [], busy: adguard.data.busy, refresh: adguard.retry }} />
  {/if}
  </CardGrid>
</div>
