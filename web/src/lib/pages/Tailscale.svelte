<script lang="ts">
import { onDestroy } from 'svelte';
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import type { TailscaleSettings } from '../api/gen';
import {
  changeTailscale,
  fetchTailscale,
  type TailscaleAction,
  tailscalePhase,
} from '../api/tailscale';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import PageHeader from '../ui/PageHeader.svelte';
import Section from '../ui/Section.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import Status from '../ui/Status.svelte';
import TailscaleOptions from './TailscaleOptions.svelte';
import { tailscaleStates } from './tailscale-states';

const request = new AbortController();
const ts = createLoader(fetchTailscale, 10000);
const confirm = createConfirm();
let pending = $state<TailscaleAction>();
onDestroy(() => {
  ts.stop();
  request.abort();
});

async function change(action: TailscaleAction, settings?: TailscaleSettings) {
  if (pending) return;
  pending = action;
  notify.progress(
    action === 'install' ? m.tailscale_installing() : m.tailscale_applying(),
    'tailscale',
  );
  const result = await changeTailscale(action, request.signal, settings);
  if (request.signal.aborted) {
    notify.dismiss('tailscale');
    return;
  }
  pending = undefined;
  if ('error' in result) notify.error(result.error, 'tailscale');
  else notify.success(m.tailscale_done(), 'tailscale');
  await ts.retry();
}

async function logout() {
  const ok = await confirm.ask({
    title: m.tailscale_logout_title(),
    description: m.tailscale_logout_body(),
    action: m.tailscale_logout(),
    tone: 'danger',
  });
  if (ok) void change('logout');
}

const phaseStatus = tailscaleStates;
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.tailscale_title()} description={m.tailscale_page_description()} />
  <Section title={m.tailscale_status_title()}>
    <LoadState status={ts.status} retry={ts.retry} stale={ts.data !== undefined}>
      {#if ts.data}
        {@const st = ts.data}
        {@const phase = tailscalePhase(st)}
        <div class="flex flex-wrap items-center gap-2">
          <Status kind={phaseStatus[phase].kind}>{phaseStatus[phase].label()}</Status>
          {#if st.busy}<Status kind="info">{m.tailscale_busy()}</Status>{/if}
        </div>
        {#if st.installed}<p class="text-sm text-muted-foreground">{m.tailscale_version({ version: st.version })}</p>{/if}
        {#if st.installed && st.prefs.accept_dns}
          <Alert class="text-danger" role="alert">{m.tailscale_accept_dns_warning()}</Alert>
        {/if}
        {#if phase === 'absent'}
          <p class="text-muted-foreground">{m.tailscale_absent_body()}</p>
          <Button variant="outline" size="sm" type="button" class="self-start" disabled={pending !== undefined || st.busy} onclick={() => change('install')}>{m.tailscale_install()}</Button>
        {:else if phase === 'login' && st.auth_url}
          <p class="text-muted-foreground">{m.tailscale_login_body()}</p>
          <Button size="sm" class="self-start" href={st.auth_url} target="_blank" rel="noopener noreferrer">{m.tailscale_open_login()}</Button>
        {:else if phase === 'login' || phase === 'off'}
          <Button variant="outline" size="sm" type="button" class="self-start" disabled={pending !== undefined || st.busy} onclick={() => change('login')}>{phase === 'login' ? m.tailscale_login() : m.tailscale_connect()}</Button>
        {:else if phase === 'connected'}
          <div>
            <SettingsRow label={m.tailscale_name()}>
              {#snippet control()}<span class="break-all">{st.self.dns_name.replace(/\.$/, '') || st.self.host_name}</span>{/snippet}
            </SettingsRow>
            <SettingsRow label={m.tailscale_addresses()}>
              {#snippet control()}<span class="break-all">{(st.self.ips ?? []).join(', ')}</span>{/snippet}
            </SettingsRow>
            {#if st.magic_dns.enabled}
              <SettingsRow label={m.tailscale_magic_dns()}>
                {#snippet control()}<span class="break-all">{st.magic_dns.suffix}</span>{/snippet}
              </SettingsRow>
            {/if}
          </div>
          <Button variant="destructive" size="sm" type="button" class="self-start" disabled={pending !== undefined || st.busy} onclick={logout}>{m.tailscale_logout()}</Button>
        {/if}
      {/if}
    </LoadState>
  </Section>

  {#if ts.data && tailscalePhase(ts.data) === 'connected'}
    <TailscaleOptions status={ts.data} disabled={pending !== undefined || ts.data.busy} {confirm} onchange={(settings) => change('settings', settings)} refresh={() => ts.retry()} />
  {/if}
</div>

<ConfirmDialog {confirm} />
