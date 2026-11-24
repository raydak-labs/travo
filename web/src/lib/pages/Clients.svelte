<script lang="ts">
import Bookmark from '@lucide/svelte/icons/bookmark';
import Laptop from '@lucide/svelte/icons/laptop';
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import {
  blockRefusal,
  clientName,
  fetchClients,
  fetchReservations,
  kick,
  type LanClient,
  type Reservation,
  runClientEdit,
  unreserve,
} from '../api/clients';
import { setClientBlocked } from '../api/gen';
import { wake } from '../api/wol';
import type { ClientEdit } from '../clients';
import { connectionStatus } from '../clients';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import EmptyState from '../ui/EmptyState.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import PageHeader from '../ui/PageHeader.svelte';
import Section from '../ui/Section.svelte';
import Status from '../ui/Status.svelte';
import { transactionToast } from '../ui/transaction-toast.svelte';
import { controlPhase, createTunnelControl } from '../vpn-tunnel.svelte';
import ClientEditDialog from './ClientEditDialog.svelte';

const list = createLoader(fetchClients, 10000);
const reservations = createLoader(fetchReservations);
const confirm = createConfirm();
let editing = $state<LanClient>();
let busy = $state(false);
// The client the guarded block toggle acts on; set before each request, which `busy` serialises.
let target = $state<LanClient>();

function refresh() {
  void list.retry();
  void reservations.retry();
}

const block = createTunnelControl(refresh, {
  submit: (blocked, signal) =>
    setClientBlocked({ path: { mac: target?.mac ?? '' }, body: { blocked }, signal }),
  refusal: blockRefusal,
  confirmed: m.clients_block_confirmed,
  rolledBack: m.clients_block_rolled_back,
  rollbackFailed: m.clients_block_rollback_failed,
  failed: m.clients_err_failed,
  unknown: m.clients_err_failed,
});
transactionToast(
  'client-block',
  () => controlPhase(block, m.clients_blocking()),
  (ok) => void block.settle(ok),
);
onDestroy(() => {
  list.stop();
  block.stop();
});

const reservationFor = (mac: string) => reservations.data?.find((r) => r.mac === mac);

function reservationName(r: Reservation): string {
  const known = list.data?.clients.find((c) => c.mac === r.mac);
  return r.name || (known ? clientName(known) : r.mac);
}

async function toggleBlock(c: LanClient) {
  const name = clientName(c);
  const ok = await confirm.ask(
    c.blocked
      ? {
          title: m.clients_unblock_title({ name }),
          description: m.clients_unblock_body(),
          action: m.clients_unblock(),
        }
      : {
          title: m.clients_block_title({ name }),
          description: m.clients_block_body(),
          action: m.clients_block(),
          tone: 'danger',
        },
  );
  if (!ok || block.busy) return;
  target = c;
  void block.set(!c.blocked);
}

async function disconnect(c: LanClient) {
  const name = clientName(c);
  const ok = await confirm.ask({
    title: m.clients_kick_title({ name }),
    description: m.clients_kick_body(),
    action: m.clients_kick(),
    tone: 'danger',
  });
  if (!ok) return;
  busy = true;
  const result = await kick(c.mac);
  busy = false;
  if ('error' in result) notify.error(result.error);
  else notify.success(m.clients_kicked({ name }));
  refresh();
}

// Wake-on-LAN sends one packet and changes nothing: no confirm, no refresh.
async function wakeUp(mac: string, name: string) {
  busy = true;
  const result = await wake(mac);
  busy = false;
  if ('error' in result) notify.error(result.error);
  else notify.success(m.wol_sent({ target: name, interfaces: result.wake.interfaces.join(', ') }));
}

async function save(edit: ClientEdit): Promise<string | undefined> {
  const error = await runClientEdit(edit);
  refresh();
  if (!error) notify.success(m.clients_saved());
  return error;
}

async function removeReservation(r: Reservation) {
  const name = reservationName(r);
  const ok = await confirm.ask({
    title: m.clients_reservation_remove_title({ name }),
    description: m.clients_reservation_remove_body(),
    action: m.clients_reservation_remove(),
    tone: 'danger',
  });
  if (!ok) return;
  busy = true;
  const result = await unreserve(r.mac);
  busy = false;
  if ('error' in result) notify.error(result.error);
  else notify.success(m.clients_reservation_removed());
  refresh();
}
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_clients()} description={m.clients_intro()} />
  <Section title={m.clients_devices_title()}>
    <LoadState status={list.status} retry={list.retry} stale={list.data !== undefined}>
      {#if list.data?.partial}
        <p role="note"><Status kind="warn">{m.clients_partial()}</Status></p>
      {/if}
      {#if list.data?.clients.length === 0}
        <EmptyState icon={Laptop} title={m.clients_empty()} />
      {:else if list.data}
        <ul class="divide-y divide-border" aria-label={m.clients_title()}>
          {#each list.data.clients as c (c.mac)}
            {@const conn = connectionStatus(c)}
            <li class="flex flex-col gap-2 py-3 sm:flex-row sm:items-center">
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="break-all font-medium">{clientName(c)}</span>
                  <Status kind={conn.kind}>{conn.label}</Status>
                  {#if c.reserved}<Status kind="info">{m.clients_reserved()}</Status>{/if}
                  {#if c.blocked}<Status kind="danger">{m.clients_blocked()}</Status>{/if}
                </div>
                <p class="break-all font-mono text-sm text-muted-foreground">
                  {c.ip ? m.clients_details({ ip: c.ip, mac: c.mac }) : c.mac}
                  {#if c.ssid}<span class="font-sans"> {m.clients_on_ssid({ ssid: c.ssid })}</span>{/if}
                </p>
              </div>
              <div class="flex flex-wrap gap-2">
                <Button variant="outline" size="sm" type="button" disabled={busy}
                  aria-label={m.clients_edit_name({ name: clientName(c) })}
                  onclick={() => { editing = c; }}>{m.clients_edit()}</Button>
                {#if c.connection === 'offline'}
                  <Button variant="outline" size="sm" type="button" disabled={busy}
                    aria-label={m.clients_wake_name({ name: clientName(c) })}
                    onclick={() => wakeUp(c.mac, clientName(c))}>{m.clients_wake()}</Button>
                {/if}
                <!-- Danger actions last: disconnect and block cut the device off. -->
                {#if c.connection === 'wifi'}
                  <Button variant="destructive" size="sm" type="button" disabled={busy}
                    onclick={() => disconnect(c)}>{m.clients_kick()}</Button>
                {/if}
                <Button variant={c.blocked ? 'outline' : 'destructive'} size="sm" type="button" disabled={busy || block.busy}
                  onclick={() => toggleBlock(c)}>{c.blocked ? m.clients_unblock() : m.clients_block()}</Button>
              </div>
            </li>
          {/each}
        </ul>
      {/if}
    </LoadState>
  </Section>

  <Section title={m.clients_reservations()} description={m.clients_reservations_intro()}>
    <LoadState status={reservations.status} retry={reservations.retry} stale={reservations.data !== undefined}>
      {#if reservations.data?.length === 0}
        <EmptyState icon={Bookmark} title={m.clients_reservations_empty()} />
      {:else if reservations.data}
        <ul class="divide-y divide-border" aria-label={m.clients_reservations()}>
          {#each reservations.data as r (r.mac)}
            <li class="flex flex-wrap items-center gap-2 py-2">
              <span class="break-all font-medium">{reservationName(r)}</span>
              <span class="break-all font-mono text-sm text-muted-foreground">{r.ip ? m.clients_details({ ip: r.ip, mac: r.mac }) : r.mac}</span>
              <Button variant="outline" size="sm" class="ml-auto" type="button" disabled={busy}
                aria-label={m.clients_wake_name({ name: reservationName(r) })}
                onclick={() => wakeUp(r.mac, reservationName(r))}>{m.clients_wake()}</Button>
              <Button variant="destructive" size="sm" type="button" disabled={busy}
                aria-label={m.clients_reservation_remove_name({ name: reservationName(r) })}
                onclick={() => removeReservation(r)}>{m.clients_reservation_remove()}</Button>
            </li>
          {/each}
        </ul>
      {/if}
    </LoadState>
  </Section>
</div>

{#if editing}
  <ClientEditDialog client={editing} reservation={reservationFor(editing.mac)} {save}
    close={() => { editing = undefined; }} />
{/if}

<ConfirmDialog {confirm} />
