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
} from '../api/gen';
import { fetchWifi, fetchWifiScan, joinWifi } from '../api/wifi';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader, type LoadStatus } from '../ui/loader.svelte';
import { wifiScanErrorMessage } from '../wifi';
import { type JoinState, joinError, reduceJoin } from '../wifi-join';
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

import WiFiScanList from './WiFiScanList.svelte';
import WiFiSummary from './WiFiSummary.svelte';

const joinDialog = lazyPage(() => import('./WiFiJoinDialog.svelte'));

const idle: LoadStatus = { state: 'ready' };
let {
  initialScanStatus = idle,
  initialScanData,
}: { initialScanStatus?: LoadStatus; initialScanData?: ScanResult } = $props();

const summary = createLoader(fetchWifi);
const request = new AbortController();
// The optional initial values exist for deterministic SSR state tests; normal routes start idle.
// svelte-ignore state_referenced_locally
let scanStatus = $state<LoadStatus>(initialScanStatus);
// svelte-ignore state_referenced_locally
let scanData = $state<ScanResult | undefined>(initialScanData);

onDestroy(() => {
  request.abort();
  activePoll?.abort();
});

let selected = $state<Network>();
let joinTrigger = $state<HTMLElement | null>(null);
let joining = $state<JoinState>({ state: 'idle' });
let commandPending = $state(false);
let activePoll: AbortController | undefined;
let now = $state(0);
const busy = $derived(
  joining.state === 'joining' || joining.state === 'pending' || joining.state === 'restoring',
);

function failure(error: { code?: string } | undefined) {
  joining = reduceJoin(joining, { type: 'failure', code: error?.code ?? '' });
}

async function submitJoin(body: JoinWiFiData['body']) {
  joining = reduceJoin(joining, { type: 'start', ssid: body.ssid });
  try {
    const response = await joinWifi(body, request.signal);
    if (request.signal.aborted) return;
    now = performance.now();
    if (response.data) {
      joining = reduceJoin(joining, { type: 'transaction', transaction: response.data, now });
    } else {
      failure(response.error);
    }
  } catch {
    if (!request.signal.aborted) failure(undefined);
  }
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

  <Card title={m.wifi_scan_title()}>
    <LoadState status={scanStatus} retry={scan} stale={scanData !== undefined}>
      {#if scanData}
        <WiFiScanList result={scanData} join={(network) => { joinTrigger = document.activeElement instanceof HTMLElement ? document.activeElement : null; selected = network; }} {busy} />
      {:else}
        <p class="text-muted">{m.wifi_scan_idle()}</p>
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
  <PageHost load={joinDialog} props={{ network: selected, returnFocus: joinTrigger, busy: joining.state === 'joining', submit: submitJoin, close: () => { selected = undefined; } }} />
{/if}
<div aria-live="polite" class="mt-4 space-y-2 text-sm">
  {#if joining.state === 'joining'}
    <p>{m.wifi_join_connecting({ ssid: joining.ssid })}</p>
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
  {/if}
</div>
