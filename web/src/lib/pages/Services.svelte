<script lang="ts">
import { onDestroy } from 'svelte';
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import { type AdGuardAction, adguardUrl, changeAdGuard, fetchAdGuard } from '../api/adguard';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Status from '../ui/Status.svelte';
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

// The DoH card is its own chunk, shown once AdGuard Home runs.
const doh = lazyPage(() => import('./DoHCard.svelte'));

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

<div class="grid gap-4">
  <Card title={m.adguard_title()}>
    <LoadState status={adguard.status} retry={adguard.retry} stale={adguard.data !== undefined}>
      {#if adguard.data}
        {@const st = adguard.data}
        {#if st.fail_open_since}
          <Alert class="mb-3 text-danger" role="alert">
            <p class="font-semibold">{m.adguard_fail_open_title()}</p>
            <p>{m.adguard_fail_open_body()}</p>
            <Button variant="outline" size="sm" type="button" class="mt-2" disabled={pending !== undefined} onclick={() => change('dns', true)}>{m.adguard_reenable()}</Button>
          </Alert>
        {/if}
        <div class="flex flex-wrap items-center gap-2">
          {#if !st.installed}
            <Status kind="neutral">{m.adguard_state_absent()}</Status>
          {:else if st.running}
            <Status kind="ok">{m.adguard_state_running()}</Status>
          {:else}
            <Status kind="warn">{m.adguard_state_stopped()}</Status>
          {/if}
          {#if st.installed}
            <Status kind={st.default_dns ? 'ok' : 'neutral'}>{st.default_dns ? m.adguard_dns_on() : m.adguard_dns_off()}</Status>
          {/if}
          {#if st.busy}<Status kind="info">{m.adguard_busy()}</Status>{/if}
        </div>
        {#if st.installed}
          <p class="mt-2 text-muted">{m.adguard_version({ version: st.version })}</p>
          {#if st.config_error}<p class="mt-2 text-muted">{m.adguard_err_config()}</p>{/if}
          {#if st.migration_failed}<p class="mt-2 text-muted">{m.adguard_migration_failed()}</p>{/if}
          <div class="mt-3 flex flex-wrap gap-2">
            <Button variant="outline" size="sm" type="button" disabled={pending !== undefined || st.busy} onclick={() => change('service', !st.running)}>{st.running ? m.adguard_stop() : m.adguard_start()}</Button>
            {#if st.running}
              <Button variant="outline" size="sm" type="button" disabled={pending !== undefined || st.busy} onclick={() => change('dns', !st.default_dns)}>{st.default_dns ? m.adguard_dns_disable() : m.adguard_dns_enable()}</Button>
              {#if st.loopback}
                <Button variant="outline" size="sm" href={proxy} target="_blank" rel="noopener">{m.adguard_open()}</Button>
              {/if}
            {/if}
          </div>
        {:else}
          <p class="mt-2 text-muted">{m.adguard_absent_body()}</p>
          <div class="mt-3">
            <Button variant="outline" size="sm" type="button" disabled={pending !== undefined || st.busy} onclick={() => change('install', true)}>{m.adguard_install()}</Button>
          </div>
        {/if}
      {/if}
    </LoadState>
  </Card>
  {#if adguard.data?.running}
    <PageHost load={doh} />
  {/if}
</div>
