<script lang="ts">
import Bookmark from '@lucide/svelte/icons/bookmark';
import EllipsisVertical from '@lucide/svelte/icons/ellipsis-vertical';
import Laptop from '@lucide/svelte/icons/laptop';
import Trash2 from '@lucide/svelte/icons/trash-2';
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
import * as Table from '$lib/components/ui/table';
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
        <Table.Root aria-label={m.clients_title()}>
          <Table.Header class="max-sm:hidden">
            <Table.Row>
              <Table.Head>{m.clients_device()}</Table.Head>
              <Table.Head class="max-sm:hidden">{m.clients_connection()}</Table.Head>
              <Table.Head class="max-md:hidden">{m.clients_address()}</Table.Head>
              <Table.Head class="w-28"><span class="sr-only">{m.table_actions()}</span></Table.Head>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {#each list.data.clients as c (c.mac)}
              {@const conn = connectionStatus(c)}
              <Table.Row>
                <Table.Cell class="whitespace-normal">
                  <div class="flex flex-wrap items-center gap-2">
                    {#if c.alias || c.hostname}
                      <span class="font-medium break-words">{clientName(c)}</span>
                    {:else}
                      <span class="text-muted-foreground">{m.clients_unknown()}</span>
                    {/if}
                    {#if c.reserved}<Status kind="info">{m.clients_reserved()}</Status>{/if}
                    {#if c.blocked}<Status kind="danger">{m.clients_blocked()}</Status>{/if}
                  </div>
                  <div class="font-mono text-xs text-muted-foreground md:hidden">{c.ip ?? c.mac}</div>
                  <div class="mt-1 flex flex-wrap items-center gap-1 sm:hidden">
                    <Status kind={conn.kind}>{conn.label}</Status>
                    {#if c.ssid}<span class="text-xs text-muted-foreground">{m.clients_on_ssid({ ssid: c.ssid })}</span>{/if}
                  </div>
                </Table.Cell>
                <Table.Cell class="whitespace-normal max-sm:hidden">
                  <Status kind={conn.kind}>{conn.label}</Status>
                  {#if c.ssid}<span class="block text-xs text-muted-foreground">{m.clients_on_ssid({ ssid: c.ssid })}</span>{/if}
                </Table.Cell>
                <Table.Cell class="font-mono text-xs text-muted-foreground max-md:hidden">
                  {#if c.ip}<span class="block">{c.ip}</span>{/if}<span class="block">{c.mac}</span>
                </Table.Cell>
                <Table.Cell class="text-right whitespace-nowrap">
                  {#if c.blocked}
                    <Button variant="outline" size="sm" type="button" disabled={busy || block.busy}
                      onclick={() => toggleBlock(c)}>{m.clients_unblock()}</Button>
                  {/if}
                  <DropdownMenu.Root>
                    <DropdownMenu.Trigger>
                      {#snippet child({ props })}
                        <Button {...props} variant="ghost" size="icon" disabled={busy}
                          aria-label={m.table_row_menu({ name: clientName(c) })}>
                          <EllipsisVertical aria-hidden="true" size={16} />
                        </Button>
                      {/snippet}
                    </DropdownMenu.Trigger>
                    <DropdownMenu.Content align="end">
                      <DropdownMenu.Item onSelect={() => { editing = c; }}>{m.clients_edit()}</DropdownMenu.Item>
                      {#if c.connection === 'offline'}
                        <DropdownMenu.Item onSelect={() => wakeUp(c.mac, clientName(c))}>{m.clients_wake()}</DropdownMenu.Item>
                      {/if}
                      <DropdownMenu.Separator />
                      {#if c.connection === 'wifi'}
                        <DropdownMenu.Item variant="destructive" onSelect={() => disconnect(c)}>{m.clients_kick()}</DropdownMenu.Item>
                      {/if}
                      {#if !c.blocked}
                        <DropdownMenu.Item variant="destructive" disabled={block.busy} onSelect={() => toggleBlock(c)}>{m.clients_block()}</DropdownMenu.Item>
                      {/if}
                    </DropdownMenu.Content>
                  </DropdownMenu.Root>
                </Table.Cell>
              </Table.Row>
            {/each}
          </Table.Body>
        </Table.Root>
      {/if}
    </LoadState>
  </Section>

  <Section title={m.clients_reservations()} description={m.clients_reservations_intro()}>
    <LoadState status={reservations.status} retry={reservations.retry} stale={reservations.data !== undefined}>
      {#if reservations.data?.length === 0}
        <EmptyState icon={Bookmark} title={m.clients_reservations_empty()} />
      {:else if reservations.data}
        <Table.Root aria-label={m.clients_reservations()}>
          <Table.Header>
            <Table.Row>
              <Table.Head>{m.clients_device()}</Table.Head>
              <Table.Head>{m.clients_address()}</Table.Head>
              <Table.Head class="w-28"><span class="sr-only">{m.table_actions()}</span></Table.Head>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {#each reservations.data as r (r.mac)}
              <Table.Row>
                <Table.Cell class="font-medium break-all whitespace-normal">{reservationName(r)}</Table.Cell>
                <Table.Cell class="font-mono text-xs text-muted-foreground">{#if r.ip}<span class="block">{r.ip}</span>{/if}<span class="block">{r.mac}</span></Table.Cell>
                <Table.Cell class="text-right whitespace-nowrap">
                  <Button variant="outline" size="sm" type="button" disabled={busy}
                    aria-label={m.clients_wake_name({ name: reservationName(r) })}
                    onclick={() => wakeUp(r.mac, reservationName(r))}>{m.clients_wake()}</Button>
                  <Button variant="ghost" size="icon" type="button" disabled={busy}
                    aria-label={m.clients_reservation_remove_name({ name: reservationName(r) })}
                    onclick={() => removeReservation(r)}><Trash2 aria-hidden="true" size={16} /></Button>
                </Table.Cell>
              </Table.Row>
            {/each}
          </Table.Body>
        </Table.Root>
      {/if}
    </LoadState>
  </Section>
</div>

{#if editing}
  <ClientEditDialog client={editing} reservation={reservationFor(editing.mac)} {save}
    close={() => { editing = undefined; }} />
{/if}

<ConfirmDialog {confirm} />
