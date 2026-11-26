<script lang="ts">
import ArrowDown from '@lucide/svelte/icons/arrow-down';
import ArrowUp from '@lucide/svelte/icons/arrow-up';
import Plus from '@lucide/svelte/icons/plus';
import X from '@lucide/svelte/icons/x';
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import { enableFailover } from '../api/gen';
import {
  failoverRefusal,
  failoverRolledBack,
  fetchFailover,
  moveUplink,
  stopFailover,
  toggleUplink,
  UPLINKS,
  type Uplink,
  warningText,
} from '../failover';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Section from '../ui/Section.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import Status from '../ui/Status.svelte';
import { transactionToast } from '../ui/transaction-toast.svelte';
import { controlPhase, createTunnelControl } from '../vpn-tunnel.svelte';

const fo = createLoader(fetchFailover, 5000);
// The draft priority list; undefined follows the saved settings.
let draft = $state<Uplink[]>();
const order = $derived(draft ?? ((fo.data?.uplinks ?? []) as Uplink[]));
const txn = createTunnelControl(
  () => {
    draft = undefined;
    return fo.retry();
  },
  {
    submit: (_enabled, signal) => enableFailover({ body: { uplinks: order }, signal }),
    refusal: failoverRefusal,
    confirmed: m.failover_confirmed,
    rolledBack: failoverRolledBack,
    rollbackFailed: m.failover_rollback_failed,
  },
);
transactionToast(
  'failover',
  () => controlPhase(txn, m.failover_applying()),
  (ok) => void txn.settle(ok),
);
const confirm = createConfirm();
const request = new AbortController();
let stopping = $state(false);
onDestroy(() => {
  fo.stop();
  txn.stop();
  request.abort();
});

const labels: Record<Uplink, () => string> = {
  wan: m.failover_uplink_wan,
  wwan: m.failover_uplink_wwan,
  usbtether: m.failover_uplink_usb,
};

async function apply() {
  const ok = await confirm.ask({
    title: m.failover_enable_title(),
    description: m.failover_enable_help({ order: order.map((u) => labels[u]()).join(' → ') }),
    action: m.failover_enable(),
  });
  if (ok) void txn.set(true);
}

async function stop() {
  const ok = await confirm.ask({
    title: m.failover_disable_title(),
    description: m.failover_disable_help(),
    action: m.failover_disable(),
    tone: 'danger',
  });
  if (!ok || stopping) return;
  stopping = true;
  const result = await stopFailover(request.signal);
  if (request.signal.aborted) return;
  stopping = false;
  if ('error' in result) notify.error(result.error);
  else notify.success(result.message);
  await fo.retry();
}
</script>

<Section title={m.failover_title()} description={m.failover_help()}>
  {#snippet actions()}
    {#if fo.data}
      {@const st = fo.data}
      {#if st.blocked}<Status kind="warn">{m.failover_blocked()}</Status>{/if}
      {#if st.enabled && st.active && st.running}
        <Status kind="ok">{m.failover_state_on()}</Status>
      {:else if st.enabled}
        <Status kind="warn">{m.failover_state_not_running()}</Status>
      {:else}
        <Status kind="neutral">{m.failover_state_off()}</Status>
      {/if}
    {/if}
    {#if fo.data && !txn.busy}
      {#if fo.data.enabled || fo.data.running}
        <Button variant="destructive" size="sm" type="button" disabled={stopping} onclick={stop}>{m.failover_disable()}</Button>
      {/if}
      <Button variant="outline" size="sm" type="button" disabled={order.length < 2} onclick={apply}>{fo.data.enabled ? m.failover_apply() : m.failover_enable()}</Button>
    {/if}
  {/snippet}
  <LoadState status={fo.status} retry={fo.retry} stale={fo.data !== undefined}>
    {#if fo.data}
      {@const st = fo.data}
      <ol>
        {#each order as uplink, i (uplink)}
          {@const live = st.states.find((s) => s.name === uplink)}
          <li>
          <SettingsRow label={`${i + 1}. ${labels[uplink]()}`}>
            {#snippet control()}
            <Status kind={live?.up ? 'ok' : 'neutral'}>{live?.up ? m.failover_up() : m.failover_down()}</Status>
            <Button variant="ghost" size="icon" type="button" disabled={i === 0 || txn.busy} aria-label={m.failover_move_up({ uplink: labels[uplink]() })} onclick={() => (draft = moveUplink(order, i, -1))}><ArrowUp aria-hidden="true" size={16} /></Button>
            <Button variant="ghost" size="icon" type="button" disabled={i === order.length - 1 || txn.busy} aria-label={m.failover_move_down({ uplink: labels[uplink]() })} onclick={() => (draft = moveUplink(order, i, 1))}><ArrowDown aria-hidden="true" size={16} /></Button>
            <Button variant="ghost" size="icon" type="button" disabled={txn.busy} aria-label={`${m.failover_remove()} ${labels[uplink]()}`} onclick={() => (draft = toggleUplink(order, uplink))}><X aria-hidden="true" size={16} /></Button>
            {/snippet}
          </SettingsRow>
          </li>
        {/each}
      </ol>
      <div class="flex flex-wrap gap-2">
      {#each UPLINKS.filter((u) => !order.includes(u)) as uplink (uplink)}
        <Button variant="outline" size="sm" type="button" disabled={txn.busy} onclick={() => (draft = toggleUplink(order, uplink))}><Plus aria-hidden="true" />{m.failover_add({ uplink: labels[uplink]() })}</Button>
      {/each}
      </div>
      {#if st.warnings.length > 0}
        <ul class="flex flex-col gap-1">
          {#each st.warnings as w (w.section + w.reason)}<li><Status kind="warn">{warningText(w)}</Status></li>{/each}
        </ul>
      {/if}
      {#if !st.installed}<p class="text-muted-foreground">{m.failover_install_note()}</p>{/if}
    {/if}
  </LoadState>
</Section>

<ConfirmDialog {confirm} />
