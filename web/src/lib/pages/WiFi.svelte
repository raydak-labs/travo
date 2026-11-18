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
} from '../api/wifi';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader, type LoadStatus } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import { transactionToast } from '../ui/transaction-toast.svelte';
import { wifiScanErrorMessage } from '../wifi';
import { type JoinState, joinPhase, reduceJoin } from '../wifi-join';
import { savedChangeError, saveError } from '../wifi-profiles';
import { lazyPage } from './lazy';
import PageHost from './PageHost.svelte';

import WiFiReconnect from './WiFiReconnect.svelte';
import WiFiSavedUplinks from './WiFiSavedUplinks.svelte';
import WiFiScanList from './WiFiScanList.svelte';
import WiFiSummary from './WiFiSummary.svelte';

const joinDialog = lazyPage(() => import('./WiFiJoinDialog.svelte'));
const accessPoints = lazyPage(() => import('./WiFiAccessPoints.svelte'));
const uplinkMAC = lazyPage(() => import('./WiFiUplinkMAC.svelte'));

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
// While the lockout dialog is open, the refusal is the dialog's to report.
let askingLockout = $state(false);
const dialog = createConfirm();
const busy = $derived(
  joining.state === 'joining' || joining.state === 'pending' || joining.state === 'restoring',
);
transactionToast(
  'wifi-change',
  () => (askingLockout ? { phase: 'idle' } : joinPhase(joining, commandPending)),
  (ok) => void settle(ok),
);

// A refused lockout risk can be resent with acknowledge_lockout (invariants §2).
async function offerLockoutOverride(resend: () => void) {
  askingLockout = true;
  const ok = await dialog.ask({
    title: m.wifi_lockout_title(),
    description: m.wifi_lockout_body(),
    action: m.wifi_join_anyway(),
    tone: 'danger',
  });
  askingLockout = false;
  if (ok && !request.signal.aborted) resend();
}

function failure(error: { code?: string } | undefined) {
  joining = reduceJoin(joining, { type: 'failure', code: error?.code ?? '' });
}

type Send = () => Promise<{ data?: TxnBody; error?: { code?: string } }>;

async function submit(ssid: string, send: Send, ap = false, acknowledge?: () => void) {
  joining = reduceJoin(joining, { type: 'start', ssid, ap });
  try {
    const response = await send();
    if (request.signal.aborted) return;
    if (response.data) {
      const now = performance.now();
      joining = reduceJoin(joining, { type: 'transaction', transaction: response.data, now });
    } else {
      if (response.error?.code === 'wifi_lockout_risk' && acknowledge) {
        void offerLockoutOverride(acknowledge);
      }
      failure(response.error);
    }
  } catch {
    if (!request.signal.aborted) failure(undefined);
  }
}

function submitJoin(body: JoinWiFiData['body']) {
  // A join that moves the AP can be refused for lockout; "Apply anyway" resends acknowledged.
  return submit(
    body.ssid,
    () => joinWifi(body, request.signal),
    false,
    () => submit(body.ssid, () => joinWifi({ ...body, acknowledge_lockout: true }, request.signal)),
  );
}

function selectProfile(profile: WiFiProfileSummary) {
  return submit(profile.ssid, () => selectWifiProfile(profile.id, request.signal));
}

// A guarded change without lockout risk, e.g. the uplink MAC policy.
function submitChange(label: string, send: Send) {
  return submit(label, send, true);
}

// AP and guest edits: send(acknowledge) builds the request; a lockout refusal offers a resend.
function submitAP(ssid: string, send: (acknowledge: boolean) => Send) {
  return submit(ssid, send(false), true, () => submit(ssid, send(true), true));
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
  const statusTimer = setInterval(() => {
    void poll();
  }, 2000);
  return () => clearInterval(statusTimer);
});

let profileBusy = $state(false);

async function changeProfiles(
  send: () => Promise<{ data?: unknown; error?: unknown }>,
  done?: string,
) {
  if (profileBusy) return;
  profileBusy = true;
  try {
    const response = await send();
    if (request.signal.aborted) return;
    if (!response.data) {
      notify.error(savedChangeError((response.error as { code?: string } | undefined)?.code));
    } else if (done) notify.success(done);
    await profiles.retry();
  } catch {
    if (!request.signal.aborted) notify.error(savedChangeError(undefined));
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
        class="w-fit text-sm"
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
          forget={(profile) =>
            changeProfiles(
              () => forgetWifiProfile(profile.id, request.signal),
              m.wifi_saved_forgotten({ ssid: profile.ssid }),
            )}
          select={selectProfile}
        />
      {/if}
    </LoadState>
  </Card>

  <Card title={m.wifi_mac_title()}>
    {#key apVersion}
      <PageHost load={uplinkMAC} props={{ busy, apply: submitChange }} />
    {/key}
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
        class="w-fit text-sm"
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
  <PageHost load={joinDialog} props={{ network: selected, summary: summary.data, returnFocus: joinTrigger, busy: joining.state === 'joining', submit: submitJoin, save: saveProfile, close: () => { selected = undefined; } }} />
{/if}
<ConfirmDialog confirm={dialog} />
