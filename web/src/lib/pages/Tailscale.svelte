<script lang="ts">
import Download from '@lucide/svelte/icons/download';
import ExternalLink from '@lucide/svelte/icons/external-link';
import LogIn from '@lucide/svelte/icons/log-in';
import LogOut from '@lucide/svelte/icons/log-out';
import { onDestroy } from 'svelte';
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import { fetchTailscale, tailscalePhase } from '../api/tailscale';
import { m } from '../paraglide/messages.js';
import { createTailscaleControl } from '../tailscale-control.svelte';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import HeaderToggle from '../ui/HeaderToggle.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import PageHeader from '../ui/PageHeader.svelte';
import Section from '../ui/Section.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import TailscaleOptions from './TailscaleOptions.svelte';
import { tailscaleStates } from './tailscale-states';

const ts = createLoader(fetchTailscale, 10000);
const confirm = createConfirm();
const control = createTailscaleControl(() => ts.retry());
const { change } = control;
onDestroy(() => {
  ts.stop();
  control.stop();
});

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
  <PageHeader title={m.tailscale_title()} description={m.tailscale_page_description()}>
    {#snippet actions()}
      {#if ts.data}
        {@const st = ts.data}
        {@const phase = tailscalePhase(st)}
        {#if !st.installed}
          <Button variant="outline" size="sm" type="button" disabled={control.pending !== undefined || st.busy} onclick={() => change('install')}><Download aria-hidden="true" />{m.tailscale_install()}</Button>
        {:else}
          <HeaderToggle id="tailscale-running" label={m.tailscale_title()}
            status={st.busy ? m.tailscale_busy() : phaseStatus[phase].label()} kind={st.busy ? 'info' : phaseStatus[phase].kind}
            checked={st.running} disabled={control.pending !== undefined || st.busy} onchange={(on) => void change(on ? 'on' : 'off')} />
          {#if st.running}
            {#if phase === 'connected'}
              <Button variant="outline" size="sm" type="button" disabled={control.pending !== undefined || st.busy} onclick={logout}><LogOut aria-hidden="true" />{m.tailscale_logout()}</Button>
            {:else if phase === 'login' || phase === 'off'}
              <Button variant="outline" size="sm" type="button" disabled={control.pending !== undefined || st.busy} onclick={() => change('login')}><LogIn aria-hidden="true" />{phase === 'login' ? m.tailscale_login() : m.tailscale_connect()}</Button>
            {/if}
          {/if}
        {/if}
      {/if}
    {/snippet}
  </PageHeader>

  <LoadState status={ts.status} retry={ts.retry} stale={ts.data !== undefined}>
    {#if ts.data}
      {@const st = ts.data}
      {@const phase = tailscalePhase(st)}
      {#if st.installed && st.prefs.accept_dns}
        <Alert class="text-danger" role="alert">{m.tailscale_accept_dns_warning()}</Alert>
      {/if}
      {#if !st.installed}
        <Section title={m.tailscale_about_title()} description={m.tailscale_absent_body()} />
      {:else if !st.running}
        <Section title={m.tailscale_state_off()} description={m.tailscale_off_body({ version: st.version })} />
      {:else if phase === 'login' && st.auth_url}
        <Section title={m.tailscale_login()} description={m.tailscale_login_body()}>
          {#snippet actions()}
            <Button size="sm" href={st.auth_url} target="_blank" rel="noopener noreferrer">{m.tailscale_open_login()}<ExternalLink aria-hidden="true" /></Button>
          {/snippet}
        </Section>
      {:else if phase === 'connected'}
        <Section title={m.tailscale_device_title()} description={m.tailscale_version({ version: st.version })}>
          <div>
            <SettingsRow label={m.tailscale_name()}>
              {#snippet control()}<span class="text-sm break-all">{st.self.dns_name.replace(/\.$/, '') || st.self.host_name}</span>{/snippet}
            </SettingsRow>
            <SettingsRow label={m.tailscale_addresses()}>
              {#snippet control()}<span class="font-mono text-xs break-all">{(st.self.ips ?? []).join(', ')}</span>{/snippet}
            </SettingsRow>
            {#if st.magic_dns.enabled}
              <SettingsRow label={m.tailscale_magic_dns()}>
                {#snippet control()}<span class="text-sm break-all">{st.magic_dns.suffix}</span>{/snippet}
              </SettingsRow>
            {/if}
          </div>
        </Section>
      {/if}
    {/if}
  </LoadState>

  {#if ts.data && tailscalePhase(ts.data) === 'connected'}
    <TailscaleOptions status={ts.data} disabled={control.pending !== undefined || ts.data.busy} {confirm} onchange={(settings) => change('settings', settings)} refresh={() => ts.retry()} />
  {/if}
</div>

<ConfirmDialog {confirm} />
