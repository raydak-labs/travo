<script lang="ts">
import { onDestroy } from 'svelte';
import Alert from '$lib/components/ui/alert/alert.svelte';
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import { setLanAddress } from '../api/gen';
import {
  ANSWER_WAIT_MS,
  adoptedTxn,
  fetchLAN,
  type MoveOutcome,
  moveOutcome,
  moveURL,
} from '../api/lan';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Section from '../ui/Section.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import { transactionToast } from '../ui/transaction-toast.svelte';
import { controlPhase, createTunnelControl } from '../vpn-tunnel.svelte';

const loader = createLoader(fetchLAN);
const confirm = createConfirm();
const request = new AbortController();
let address = $state('');
let filled = false;
let sending = $state(false);
// Set once the change left this page: where to go and when the window closes.
let moving = $state<{ url: string; old: string; deadline: number }>();
let now = $state(performance.now());
const tick = setInterval(() => (now = performance.now()), 1000);

// On the new address the pending change is adopted by the shared transaction control, which
// shows the confirm window with Keep change / Roll back.
const txn = createTunnelControl(() => loader.retry(), {
  submit: async () => ({
    data: loader.data?.pending ? adoptedTxn(loader.data.pending) : undefined,
  }),
  refusal: () => ({}),
  confirmed: m.lan_confirmed,
  rolledBack: m.lan_rolled_back,
  rollbackFailed: m.lan_rollback_failed,
  failed: m.lan_failed,
});
transactionToast(
  'lan',
  () => controlPhase(txn, m.lan_applying()),
  (ok) => void txn.settle(ok),
);
onDestroy(() => {
  clearInterval(tick);
  request.abort();
  txn.stop();
});

let adopted = '';
$effect(() => {
  const data = loader.data;
  if (!data) return;
  if (!filled) {
    filled = true;
    address = data.address;
  }
  if (data.pending && data.pending.id !== adopted && !txn.busy) {
    adopted = data.pending.id;
    void txn.set(true);
  }
});

const left = $derived(moving ? Math.max(0, Math.ceil((moving.deadline - now) / 1000)) : 0);

async function send(next: string, old: string) {
  sending = true;
  const started = performance.now();
  const put = setLanAddress({ body: { address: next }, signal: request.signal });
  const wait = new Promise<'lost'>((resolve) => setTimeout(() => resolve('lost'), ANSWER_WAIT_MS));
  let outcome: MoveOutcome;
  try {
    outcome = moveOutcome(await Promise.race([put, wait]));
  } catch {
    outcome = moveOutcome('lost');
  }
  sending = false;
  if (request.signal.aborted) return;
  if (outcome.kind === 'moving') {
    moving = { url: moveURL(next, location), old, deadline: started + outcome.seconds * 1000 };
  } else if (outcome.kind === 'rolled_back') {
    notify.error(m.lan_rolled_back());
    void loader.retry();
  } else if (outcome.error) {
    notify.error(outcome.error);
  } else if (outcome.message) {
    notify.info(outcome.message);
  }
}

async function save(event: SubmitEvent) {
  event.preventDefault();
  const current = loader.data;
  if (!current || sending || moving || txn.busy) return;
  const next = address.trim();
  const ok = await confirm.ask({
    title: m.lan_confirm_title(),
    description: m.lan_confirm({ url: moveURL(next, location), old: current.address }),
    action: m.lan_apply(),
    tone: 'danger',
  });
  if (ok) void send(next, current.address);
}
const formId = $props.id();
</script>

<Section title={m.lan_title()} description={m.lan_intro()}>
  {#if moving}
    <Alert class="status status-warn flex flex-col gap-2" role="status">
      {#if left > 0}
        <span>{m.lan_moving({ seconds: left })}</span>
        <a class="font-medium underline" href={moving.url}>{moving.url}</a>
        <span class="text-sm">{m.lan_moving_fallback({ old: moving.old })}</span>
      {:else}
        <span>{m.lan_moving_over({ old: moving.old })}</span>
      {/if}
    </Alert>
  {:else}
    <LoadState status={loader.status} retry={loader.retry} stale={loader.data !== undefined}>
      {#if loader.data}
        {#if loader.data.pending}
          <Alert class="status status-warn" role="status">
            {m.lan_pending({ address: loader.data.pending.address })}
          </Alert>
        {/if}
        <form id={formId} onsubmit={save}>
          <SettingsRow label={m.lan_address()} for="{formId}-address" description={m.lan_subnet({ subnet: loader.data.subnet })}>
            {#snippet control()}
              <Input id="{formId}-address" class="w-64" bind:value={address} required maxlength={18} placeholder="192.168.8.1/24"
                autocomplete="off" spellcheck={false} disabled={sending || txn.busy} />
            {/snippet}
          </SettingsRow>
        </form>
      {/if}
    </LoadState>
  {/if}
  {#snippet footer()}
    {#if !moving}
      <Button variant="default" type="submit" form={formId} disabled={sending || txn.busy || !loader.data}>
        {sending ? m.lan_applying() : m.lan_apply()}
      </Button>
    {/if}
  {/snippet}
</Section>

<ConfirmDialog {confirm} />
