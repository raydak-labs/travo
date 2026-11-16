<script lang="ts">
import { onDestroy, onMount } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import {
  confirmTransaction,
  getTransaction,
  type JoinWiFiData,
  type Network,
  rollbackTransaction,
  type ScanResult,
  type TxnBody,
  type UpdateWiFiAccessPointData,
  type WiFiProfileSummary,
} from '../api/gen';
import {
  fetchWifi,
  fetchWifiProfiles,
  fetchWifiReconnect,
  fetchWifiScan,
  forgetWifiProfile,
  joinWifi,
  reorderWifiProfiles,
  saveWifiProfile,
  selectWifiProfile,
  updateAccessPoint,
} from '../api/wifi';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader, type LoadStatus } from '../ui/loader.svelte';
import { wifiScanErrorMessage } from '../wifi';
import { type JoinState, joinError, reduceJoin } from '../wifi-join';
import { savedChangeError, saveError } from '../wifi-profiles';
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

import WiFiReconnect from './WiFiReconnect.svelte';
import WiFiSavedUplinks from './WiFiSavedUplinks.svelte';
import WiFiScanList from './WiFiScanList.svelte';
import WiFiSummary from './WiFiSummary.svelte';

const joinDialog = lazyPage(() => import('./WiFiJoinDialog.svelte'));
const accessPoints = lazyPage(() => import('./WiFiAccessPoints.svelte'));

const idle: LoadStatus = { state: 'ready' };
let {
  initialScanStatus = idle,
  initialScanData,
}: { initialScanStatus?: LoadStatus; initialScanData?: ScanResult } = $props();

const summary = createLoader(fetchWifi);
const profiles = createLoader(fetchWifiProfiles);
// The worker ticks every minute; a 30 s poll shows each attempt within one tick.
const reconnect = createLoader(fetchWifiReconnect, 30_000);
const request = new AbortController();
// The optional initial values exist for deterministic SSR state tests; normal routes start idle.
// svelte-ignore state_referenced_locally
let scanStatus = $state<LoadStatus>(initialScanStatus);
// svelte-ignore state_referenced_locally
let scanData = $state<ScanResult | undefined>(initialScanData);

onDestroy(() => {
  reconnect.stop();
  request.abort();
  activePoll?.abort();
});

let selected = $state<Network>();
let joinTrigger = $state<HTMLElement | null>(null);
let joining = $state<JoinState>({ state: 'idle' });
let commandPending = $state(false);
let activePoll: AbortController | undefined;
let apVersion = $state(0);
// Resends the last refused change with acknowledge_lockout (invariants §2).
let anyway = $state<() => void>();
let now = $state(0);
const busy = $derived(
  joining.state === 'joining' || joining.state === 'pending' || joining.state === 'restoring',
);

function failure(error: { code?: string } | undefined) {
  joining = reduceJoin(joining, { type: 'failure', code: error?.code ?? '' });
}

type Send = () => Promise<{ data?: TxnBody; error?: { code?: string } }>;

async function submit(ssid: string, send: Send, ap = false, acknowledge?: () => void) {
  joining = reduceJoin(joining, { type: 'start', ssid, ap });
  anyway = undefined;
  try {
    const response = await send();
    if (request.signal.aborted) return;
    now = performance.now();
    if (response.data) {
      joining = reduceJoin(joining, { type: 'transaction', transaction: response.data, now });
    } else {
      failure(response.error);
      if (response.error?.code === 'wifi_lockout_risk') anyway = acknowledge;
    }
  } catch {
    if (!request.signal.aborted) failure(undefined);
  }
}

function submitJoin(body: JoinWiFiData['body']) {
  return submit(body.ssid, () => joinWifi(body, request.signal));
}

function selectProfile(profile: WiFiProfileSummary) {
  return submit(profile.ssid, () => selectWifiProfile(profile.id, request.signal));
}

function submitAP(section: string, body: UpdateWiFiAccessPointData['body']) {
  return submit(
    body.ssid,
    () => updateAccessPoint(section, body, request.signal),
    true,
    () => submitAP(section, { ...body, acknowledge_lockout: true }),
  );
}

async function poll() {
  if (
    activePoll ||
    commandPending ||
    (joining.state !== 'pending' && joining.state !== 'restoring')
  )
    return;
  const id = joining.transaction.id;
  const pollRequest = new AbortController();
  activePoll = pollRequest;
  const timeout = setTimeout(() => pollRequest.abort(), 4000);
  try {
    const response = await getTransaction({ path: { id }, signal: pollRequest.signal });
    if (
      pollRequest.signal.aborted ||
      activePoll !== pollRequest ||
      !('transaction' in joining) ||
      joining.transaction.id !== id
    )
      return;
    if (response.data) {
      joining = reduceJoin(joining, {
        type: 'transaction',
        transaction: response.data,
        now: performance.now(),
      });
      // An expired window rolls back on its own; the connected uplink may have changed.
      if (joining.state !== 'pending' && joining.state !== 'restoring') void profiles.retry();
    } else if (response.response?.status === 404) {
      failure(response.error);
    }
  } catch {
    // A transient read failure does not prove an outcome.
  } finally {
    clearTimeout(timeout);
    if (activePoll === pollRequest) activePoll = undefined;
  }
}

async function settle(confirm: boolean) {
  if (commandPending || joining.state !== 'pending') return;
  commandPending = true;
  activePoll?.abort();
  activePoll = undefined;
  const operation = confirm ? confirmTransaction : rollbackTransaction;
  try {
    const response = await operation({
      path: { id: joining.transaction.id },
      signal: request.signal,
    });
    if (request.signal.aborted) return;
    if (response.data) {
      joining = reduceJoin(joining, {
        type: 'transaction',
        transaction: response.data,
        now: performance.now(),
      });
      summary.retry();
      void profiles.retry();
      apVersion++;
    } else if (response.error?.code === 'restoring' || response.error?.code === 'rollback_failed') {
      joining = { ...joining, state: response.error.code };
    }
  } catch {
    // Keep reading the transaction until the daemon reports the result.
  } finally {
    commandPending = false;
  }
}

onMount(() => {
  const timer = setInterval(() => {
    now = performance.now();
  }, 1000);
  const statusTimer = setInterval(() => {
    void poll();
  }, 2000);
  return () => {
    clearInterval(timer);
    clearInterval(statusTimer);
  };
});

let profileBusy = $state(false);
let profileError = $state('');

async function changeProfiles(send: () => Promise<{ data?: unknown; error?: unknown }>) {
  if (profileBusy) return;
  profileBusy = true;
  profileError = '';
  try {
    const response = await send();
    if (request.signal.aborted) return;
    if (!response.data) {
      profileError = savedChangeError((response.error as { code?: string } | undefined)?.code);
    }
    await profiles.retry();
  } catch {
    if (!request.signal.aborted) profileError = savedChangeError(undefined);
  } finally {
    profileBusy = false;
  }
}

// Saving stores credentials only; the live uplink is untouched.
async function saveProfile(body: JoinWiFiData['body']): Promise<string | undefined> {
  try {
    const response = await saveWifiProfile(body, request.signal);
    if (!response.data) return saveError((response.error as { code?: string } | undefined)?.code);
    void profiles.retry();
    return undefined;
  } catch {
    return saveError(undefined);
  }
}

async function scan() {
  if (scanStatus.state === 'loading') return;
  scanStatus = { state: 'loading' };
  try {
    const result = await fetchWifiScan(request.signal);
    if (request.signal.aborted) return;
    scanData = result;
    scanStatus = { state: 'ready' };
  } catch (error) {
    if (!request.signal.aborted) {
      scanStatus = { state: 'error', message: wifiScanErrorMessage(error) };
    }
  }
}
</script>

<div class="grid gap-4">
  <Card title={m.wifi_summary_title()}>
    <LoadState status={summary.status} retry={summary.retry} stale={summary.data !== undefined}>
      {#if summary.data}
        <WiFiSummary data={summary.data} />
      {/if}
    </LoadState>
    {#if summary.status.state === 'ready'}
      <Button variant="outline" size="sm"
        type="button"
        class="mt-3 text-sm"
        onclick={summary.retry}
      >
        {m.wifi_refresh()}
      </Button>
    {/if}
  </Card>

  <Card title={m.wifi_saved_title()}>
    <WiFiReconnect status={reconnect.status} data={reconnect.data} profiles={profiles.data?.profiles ?? []}
      scan={scanData} joinBusy={busy} select={selectProfile} />
    <LoadState status={profiles.status} retry={profiles.retry} stale={profiles.data !== undefined}>
      {#if profiles.data}
        <WiFiSavedUplinks
          list={profiles.data}
          busy={profileBusy}
          joinBusy={busy}
          reorder={(ids) => changeProfiles(() => reorderWifiProfiles(ids, request.signal))}
          forget={(profile) => changeProfiles(() => forgetWifiProfile(profile.id, request.signal))}
          select={selectProfile}
        />
      {/if}
    </LoadState>
    {#if profileError}
      <p class="mt-2 text-sm text-danger" role="alert">{profileError}</p>
    {/if}
  </Card>

  <Card title={m.wifi_aps_title()}>
    {#key apVersion}
      <PageHost load={accessPoints} props={{ busy, apply: submitAP }} />
    {/key}
  </Card>

  <Card title={m.wifi_scan_title()}>
    <LoadState status={scanStatus} retry={scan} stale={scanData !== undefined}>
      {#if scanData}
        <WiFiScanList result={scanData} join={(network) => { joinTrigger = document.activeElement instanceof HTMLElement ? document.activeElement : null; selected = network; }} {busy} />
      {:else}
        <p class="text-muted-foreground">{m.wifi_scan_idle()}</p>
      {/if}
    </LoadState>
    {#if scanStatus.state !== 'error'}
      <Button variant="outline" size="sm"
        type="button"
        class="mt-3 text-sm"
        disabled={scanStatus.state === 'loading'}
        aria-busy={scanStatus.state === 'loading'}
        onclick={scan}
      >
        {scanStatus.state === 'loading'
          ? m.wifi_scanning()
          : scanData
            ? m.wifi_scan_again()
            : m.wifi_scan()}
      </Button>
    {/if}
  </Card>
</div>


{#if selected}
  <PageHost load={joinDialog} props={{ network: selected, returnFocus: joinTrigger, busy: joining.state === 'joining', submit: submitJoin, save: saveProfile, close: () => { selected = undefined; } }} />
{/if}
<div aria-live="polite" class="mt-4 space-y-2 text-sm">
  {#if joining.state === 'joining'}
    <p>{joining.ap ? m.wifi_ap_applying({ ssid: joining.ssid }) : m.wifi_join_connecting({ ssid: joining.ssid })}</p>
  {:else if joining.state === 'pending'}
    <p>{m.wifi_join_pending({ seconds: Math.max(0, Math.ceil((joining.deadline - now) / 1000)) })}</p>
    <div class="flex gap-2">
      <Button variant="outline" type="button" disabled={commandPending} onclick={() => settle(true)}>{m.wifi_join_confirm()}</Button>
      <Button variant="outline" type="button" disabled={commandPending} onclick={() => settle(false)}>{m.wifi_join_rollback()}</Button>
    </div>
  {:else if joining.state === 'restoring'}
    <p>{m.wifi_join_restoring()}</p>
  {:else if joining.state === 'confirmed'}
    <p>{m.wifi_join_confirmed()}</p>
  {:else if joining.state === 'rolled_back'}
    <p>{m.wifi_join_rolled_back()}</p>
  {:else if joining.state === 'rollback_failed'}
    <p class="text-danger">{m.wifi_join_rollback_failed()}</p>
  {:else if joining.state === 'failed'}
    <p class="text-danger">{joinError(joining.code)}</p>
    {#if anyway}
      <Button variant="outline" type="button" onclick={anyway}>{m.wifi_join_anyway()}</Button>
    {/if}
  {/if}
</div>
