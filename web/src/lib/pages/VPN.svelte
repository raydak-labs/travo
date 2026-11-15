<script lang="ts">
import { onDestroy, onMount } from 'svelte';
import '../api/client';
import {
  confirmTransaction,
  deleteVpnProfile,
  getTransaction,
  getVpnProfiles,
  postVpnProfile,
  rollbackTransaction,
  selectVpnProfile,
  setVpnTunnel,
  type TxnBody,
} from '../api/gen';
import { fetchVpnStatus } from '../api/vpn-status';
import { formatUptime } from '../format';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import Status from '../ui/Status.svelte';

const request = new AbortController();
const status = createLoader(fetchVpnStatus, 10000);
const profiles = createLoader(async (signal) => {
  const result = await getVpnProfiles({ signal });
  if (!result.data) throw new Error(m.vpn_unavailable());
  return result.data;
});
let name = $state('');
let config = $state('');
let pending = $state(false);
let error = $state('');
let removing = $state<string>();
onDestroy(() => {
  status.stop();
  profiles.stop();
  request.abort();
  config = '';
});

async function change(action: 'import' | 'select' | 'delete', id = '') {
  if (pending) return;
  pending = true;
  error = '';
  try {
    const options = { signal: request.signal };
    const response =
      action === 'import'
        ? await postVpnProfile({ ...options, body: { name, config } })
        : action === 'select'
          ? await selectVpnProfile({ ...options, path: { id } })
          : await deleteVpnProfile({ ...options, path: { id } });
    if (request.signal.aborted) return;
    if (!response.data) {
      const code = (response.error as { code?: string } | undefined)?.code;
      error =
        code === 'invalid_profile'
          ? m.vpn_invalid()
          : code === 'profile_limit'
            ? m.vpn_limit()
            : m.vpn_save_failed();
      return;
    }
    if (action === 'import') {
      name = '';
      config = '';
    }
    removing = undefined;
    await profiles.retry();
  } catch {
    if (!request.signal.aborted) error = m.vpn_save_failed();
  } finally {
    if (!request.signal.aborted) pending = false;
  }
}

let applying = $state(false);
let settling = $state(false);
let transaction = $state<TxnBody>();
let tunnelMessage = $state('');
let tunnelError = $state('');
let statusPoll: AbortController | undefined;
const hasTransaction = $derived(
  transaction?.state === 'pending' || transaction?.state === 'restoring',
);
const tunnelBusy = $derived(applying || settling || hasTransaction);

function acceptTunnelResult(result: TxnBody) {
  transaction = result;
  if (result.state === 'confirmed') tunnelMessage = m.vpn_tunnel_confirmed();
  if (result.state === 'rolled_back') tunnelMessage = m.vpn_tunnel_rolled_back();
  if (result.state === 'rollback_failed') tunnelError = m.vpn_tunnel_rollback_failed();
  if (result.state !== 'pending' && result.state !== 'restoring') void status.retry();
}

async function setTunnel(enabled: boolean) {
  if (tunnelBusy) return;
  applying = true;
  transaction = undefined;
  tunnelMessage = '';
  tunnelError = '';
  try {
    const response = await setVpnTunnel({ body: { enabled }, signal: request.signal });
    if (request.signal.aborted) return;
    if (response.data) {
      acceptTunnelResult(response.data);
    } else {
      switch (response.error?.code) {
        case 'vpn_profile_required':
          tunnelError = m.vpn_tunnel_profile_required();
          break;
        case 'vpn_no_change':
          tunnelMessage = m.vpn_tunnel_no_change();
          break;
        case 'transaction_pending':
          tunnelError = m.vpn_tunnel_busy();
          break;
        default:
          tunnelError = m.vpn_tunnel_failed();
      }
    }
  } catch {
    if (!request.signal.aborted) tunnelError = m.vpn_tunnel_failed();
  } finally {
    applying = false;
  }
}

async function pollTunnel() {
  if (statusPoll || settling || !hasTransaction || !transaction) return;
  const id = transaction.id;
  const poll = new AbortController();
  statusPoll = poll;
  const timer = setTimeout(() => poll.abort(), 4000);
  try {
    const response = await getTransaction({ path: { id }, signal: poll.signal });
    if (poll.signal.aborted || statusPoll !== poll || transaction?.id !== id) return;
    if (response.data) acceptTunnelResult(response.data);
    else if (response.response?.status === 404) {
      transaction = undefined;
      tunnelError = m.vpn_tunnel_unknown_result();
      void status.retry();
    }
  } catch {
    // A failed status read gives no evidence about the transaction outcome.
  } finally {
    clearTimeout(timer);
    if (statusPoll === poll) statusPoll = undefined;
  }
}

async function settleTunnel(confirm: boolean) {
  if (settling || transaction?.state !== 'pending') return;
  settling = true;
  statusPoll?.abort();
  statusPoll = undefined;
  try {
    const operation = confirm ? confirmTransaction : rollbackTransaction;
    const response = await operation({ path: { id: transaction.id }, signal: request.signal });
    if (request.signal.aborted) return;
    if (response.data) acceptTunnelResult(response.data);
    else if (response.error?.code === 'restoring' || response.error?.code === 'rollback_failed') {
      acceptTunnelResult({ ...transaction, state: response.error.code });
    }
  } catch {
    // Resume polling after a failed command.
  } finally {
    settling = false;
  }
}

onMount(() => {
  const timer = setInterval(() => void pollTunnel(), 2000);
  return () => {
    clearInterval(timer);
    statusPoll?.abort();
  };
});

const tunnelStates = {
  disabled: { kind: 'neutral', label: m.vpn_state_disabled },
  configured: { kind: 'info', label: m.vpn_state_configured },
  enabled_not_up: { kind: 'warn', label: m.vpn_state_enabled_not_up },
  no_handshake: { kind: 'warn', label: m.vpn_state_no_handshake },
  connected: { kind: 'ok', label: m.vpn_state_connected },
} as const;
</script>

<div class="grid gap-4">
  <Card title={m.vpn_tunnel()}>
    <LoadState status={status.status} retry={status.retry} stale={status.data !== undefined}>
      {#if status.data}
        {@const snapshot = status.data}
        {@const badge = tunnelStates[snapshot.state]}
        <div class="flex flex-wrap items-center gap-2">
          <Status kind={badge.kind}>{badge.label()}</Status>
          {#if snapshot.stale}<Status kind="stale">{m.load_stale()}</Status>{/if}
        </div>
        {#if snapshot.handshake_age_seconds !== null}
          <p class="mt-2 text-muted">{m.vpn_handshake_age({ age: formatUptime(snapshot.handshake_age_seconds) })}</p>
        {/if}
      {/if}
    </LoadState>
    <div class="mt-3 flex flex-wrap gap-2">
      {#if transaction?.state === 'pending'}
        <button type="button" class="rounded-md border border-border px-3 py-1" disabled={settling} onclick={() => settleTunnel(true)}>{m.vpn_tunnel_confirm()}</button>
        <button type="button" class="rounded-md border border-border px-3 py-1" disabled={settling} onclick={() => settleTunnel(false)}>{m.vpn_tunnel_rollback()}</button>
      {:else if status.data && !tunnelBusy}
        {@const enabled = !['disabled', 'configured'].includes(status.data.state)}
        <button type="button" class="rounded-md border border-border px-3 py-1 disabled:opacity-60" disabled={status.data.stale || (!enabled && !profiles.data?.selected_id)} onclick={() => setTunnel(!enabled)}>{enabled ? m.vpn_tunnel_disable() : m.vpn_tunnel_enable()}</button>
        {#if enabled && profiles.data?.selected_id}
          <button type="button" class="rounded-md border border-border px-3 py-1 disabled:opacity-60" disabled={status.data.stale} onclick={() => setTunnel(true)}>{m.vpn_tunnel_apply()}</button>
        {/if}
      {/if}
    </div>
    {#if applying}
      <p role="status" class="mt-2">{m.vpn_tunnel_applying()}</p>
    {:else if transaction?.state === 'pending'}
      <p role="status" class="mt-2">{m.vpn_tunnel_pending()}</p>
    {:else if transaction?.state === 'restoring'}
      <p role="status" class="mt-2">{m.vpn_tunnel_restoring()}</p>
    {:else if tunnelMessage}
      <p role="status" class="mt-2">{tunnelMessage}</p>
    {/if}
    {#if tunnelError}<p role="alert" class="mt-2 text-danger">{tunnelError}</p>{/if}
  </Card>

  <Card title={m.vpn_saved_profiles()}>
    <LoadState status={profiles.status} retry={profiles.retry}>
      {#if profiles.data}
        {#if profiles.data.profiles.length === 0}
          <p class="text-muted">{m.vpn_no_profiles()}</p>
        {:else}
          <ul class="space-y-3">
            {#each profiles.data.profiles as profile (profile.id)}
              <li class="flex flex-wrap items-center justify-between gap-2">
                <span class="break-all">{profile.name}</span>
                <div class="flex flex-wrap items-center gap-2">
                  {#if profile.id === profiles.data.selected_id}
                    <span class="text-muted">{m.vpn_selected()}</span>
                  {:else}
                    <button type="button" class="rounded-md border border-border px-3 py-1 text-sm hover:bg-bg disabled:opacity-60" disabled={pending} onclick={() => change('select', profile.id)}>{m.vpn_select()}</button>
                  {/if}
                  {#if removing === profile.id}
                    <button type="button" class="rounded-md border border-border px-3 py-1 text-sm hover:bg-bg disabled:opacity-60" disabled={pending} onclick={() => change('delete', profile.id)}>{m.vpn_confirm_delete()}</button>
                    <button type="button" class="rounded-md border border-border px-3 py-1 text-sm hover:bg-bg disabled:opacity-60" disabled={pending} onclick={() => { removing = undefined; }}>{m.vpn_cancel()}</button>
                  {:else}
                    <button type="button" class="rounded-md border border-border px-3 py-1 text-sm hover:bg-bg disabled:opacity-60" disabled={pending} onclick={() => { removing = profile.id; }}>{m.vpn_delete()}</button>
                  {/if}
                </div>
              </li>
            {/each}
          </ul>
        {/if}
      {/if}
    </LoadState>
  </Card>

  <Card title={m.vpn_import()}>
    <form class="grid gap-3" autocomplete="off" onsubmit={(event) => { event.preventDefault(); void change('import'); }}>
      <label class="grid gap-1">
        <span>{m.vpn_name()}</span>
        <input class="rounded-md border border-border bg-bg px-3 py-2" bind:value={name} required disabled={pending} />
      </label>
      <label class="grid gap-1">
        <span>{m.vpn_config()}</span>
        <textarea class="w-full rounded-md border border-border bg-bg px-3 py-2 font-mono" rows="6" bind:value={config} required disabled={pending} spellcheck="false"></textarea>
      </label>
      <button type="submit" class="w-fit rounded-md bg-accent px-4 py-2 text-accent-contrast disabled:opacity-60" disabled={pending || !name.trim() || !config.trim()}>{m.vpn_import()}</button>
    </form>
  </Card>
  {#if error}<p role="alert" class="text-danger">{error}</p>{/if}
</div>
