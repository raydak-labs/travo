<script lang="ts">
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import { type DashboardConnectivity, enableCaptiveDnsBypass, restoreCaptiveDns } from '../api/gen';
import { captiveError, formatCountdown } from '../captive';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import Status from '../ui/Status.svelte';

// Shown only when an action exists (captiveCardVisible); refresh re-reads connectivity.
let { info, refresh }: { info: DashboardConnectivity; refresh: () => void } = $props();

let busy = $state(false);
let error = $state('');

async function run(action: () => Promise<{ data?: unknown; error?: { code?: string } }>) {
  if (busy) return;
  busy = true;
  error = '';
  try {
    const response = await action();
    if (!response.data) error = captiveError(response.error?.code);
  } catch {
    error = captiveError(undefined);
  } finally {
    busy = false;
    refresh();
  }
}
</script>

<Card title={m.captive_title()}>
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
    <div class="flex flex-wrap gap-2">
      {#if info.portal_url}
        <Button size="sm" href={info.portal_url} target="_blank" rel="noopener noreferrer">
          {m.captive_open_login()}
        </Button>
      {/if}
      {#if info.captive.dns_bypass}
        <Button size="sm" variant="outline" type="button" disabled={busy} onclick={() => run(() => restoreCaptiveDns())}>
          {m.captive_dns_restore()}
        </Button>
      {:else if !info.captive.pending}
        <Button size="sm" variant="outline" type="button" disabled={busy} onclick={() => run(() => enableCaptiveDnsBypass())}>
          {m.captive_dns_bypass()}
        </Button>
      {/if}
    </div>
    {#if error}<Alert class="text-danger">{error}</Alert>{/if}
  </div>
</Card>
