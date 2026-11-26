<script lang="ts">
import Button from '$lib/components/ui/button/button.svelte';
import {
  autoAcceptCaptivePortal,
  type DashboardConnectivity,
  enableCaptiveDnsBypass,
  renewCaptiveDhcp,
  restoreCaptiveDns,
} from '../api/gen';
import { acceptError, acceptMessage, captiveError, formatCountdown, renewError } from '../captive';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import { notify } from '../ui/notify';
import Section from '../ui/Section.svelte';
import Status from '../ui/Status.svelte';

// Shown only when an action exists (captiveCardVisible); refresh re-reads connectivity.
let { info, refresh }: { info: DashboardConnectivity; refresh: () => void } = $props();

let busy = $state(false);
const confirm = createConfirm();

async function run(
  action: () => Promise<{ data?: unknown; error?: { code?: string } }>,
  done: string,
  describe = captiveError,
) {
  if (busy) return;
  busy = true;
  try {
    const response = await action();
    if (response.data) notify.success(done);
    else notify.error(describe(response.error?.code));
  } catch {
    notify.error(describe(undefined));
  } finally {
    busy = false;
    refresh();
  }
}

// The walk can take up to 30 s: one toast turns from progress into its result.
async function autoLogin() {
  if (busy) return;
  busy = true;
  const id = 'captive-accept';
  notify.progress(m.captive_accept_running(), id);
  try {
    const response = await autoAcceptCaptivePortal();
    if (response.data) {
      const result = acceptMessage(response.data.outcome);
      notify[result.kind](result.text, id);
    } else notify.error(acceptError(response.error?.code), id);
  } catch {
    notify.error(acceptError(undefined), id);
  } finally {
    busy = false;
    refresh();
  }
}

// The bypass pauses filtering and VPN DNS, so it is asked first; restoring is the safe side.
async function bypass() {
  const ok = await confirm.ask({
    title: m.captive_bypass_title(),
    description: m.captive_bypass_body(),
    action: m.captive_dns_bypass(),
  });
  if (ok) void run(() => enableCaptiveDnsBypass(), m.captive_dns_bypassed());
}
</script>

<Section title={m.captive_title()}>
  <div class="flex flex-col items-start gap-3">
    {#if info.internet === 'captive'}
      <Status kind="warn">{m.dashboard_internet_captive()}</Status>
      <p>{m.captive_login_needed()}</p>
    {:else if info.internet === 'offline'}
      <Status kind="danger">{m.dashboard_internet_offline()}</Status>
      <p>{m.captive_offline_hint()}</p>
    {/if}
    {#if info.captive.pending}
      <Status kind="danger">{m.captive_dns_pending()}</Status>
    {:else if info.captive.dns_bypass}
      <p>
        <Status kind="info">{m.captive_dns_active()}</Status>
        {info.captive.restore_in_seconds === null
          ? m.captive_dns_restores_soon()
          : m.captive_dns_restores_in({ time: formatCountdown(info.captive.restore_in_seconds) })}
      </p>
    {/if}
    {#if info.captive.dhcp_renew === 'blocked'}
      <Status kind="warn">{m.captive_renew_blocked()}</Status>
    {/if}
    <div class="flex flex-wrap gap-2">
      {#if info.internet === 'captive'}
        <Button size="sm" type="button" disabled={busy} onclick={autoLogin}>{m.captive_accept()}</Button>
      {/if}
      {#if info.portal_url}
        <Button size="sm" variant="outline" href={info.portal_url} target="_blank" rel="noopener noreferrer">
          {m.captive_open_login()}
        </Button>
      {/if}
      {#if info.captive.pending}
        <Button size="sm" variant="outline" type="button" disabled={busy} onclick={() => run(() => restoreCaptiveDns(), m.captive_dns_restored())}>
          {m.captive_dns_retry()}
        </Button>
      {:else if info.captive.dns_bypass}
        <Button size="sm" variant="outline" type="button" disabled={busy} onclick={() => run(() => restoreCaptiveDns(), m.captive_dns_restored())}>
          {m.captive_dns_restore()}
        </Button>
      {:else}
        <Button size="sm" variant="outline" type="button" disabled={busy} onclick={bypass}>
          {m.captive_dns_bypass()}
        </Button>
      {/if}
      {#if info.captive.dhcp_renew !== 'not_applicable'}
        <Button size="sm" variant="outline" type="button" disabled={busy} onclick={() => run(() => renewCaptiveDhcp(), m.captive_renew_done(), renewError)}>
          {info.captive.dhcp_renew === 'blocked' ? m.captive_renew_retry() : m.captive_renew()}
        </Button>
      {/if}
    </div>
  </div>
</Section>

<ConfirmDialog {confirm} />
