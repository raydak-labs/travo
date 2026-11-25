<script lang="ts">
import RotateCw from '@lucide/svelte/icons/rotate-cw';
import Shield from '@lucide/svelte/icons/shield';
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import * as Table from '$lib/components/ui/table';
import {
  fetchInterfaces,
  fetchZones,
  interfaceAddresses,
  interfaceStatus,
  type Restartable,
  restartable,
  restartUplink,
} from '../api/network-view';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import EmptyState from '../ui/EmptyState.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Section from '../ui/Section.svelte';
import Status from '../ui/Status.svelte';

// Read-only view except the uplink restart; edits stay in LuCI. Uptime moves, so the list refreshes.
const ifaces = createLoader(fetchInterfaces, 10000);
const zones = createLoader(fetchZones, 30000);
const confirm = createConfirm();
let restarting = $state<string>();
onDestroy(() => {
  ifaces.stop();
  zones.stop();
});

async function restart(name: Restartable) {
  const ok = await confirm.ask({
    title: m.netview_restart_title({ name }),
    description: m.netview_restart_body(),
    action: m.netview_restart(),
  });
  if (!ok || restarting) return;
  restarting = name;
  const id = `restart-${name}`;
  notify.progress(m.netview_restarting({ name }), id);
  let result = await restartUplink(name);
  if ('lockout' in result) {
    notify.dismiss(id);
    // A refused lockout risk can be resent with acknowledge_lockout (invariants §2).
    const anyway = await confirm.ask({
      title: m.netview_restart_lockout_title(),
      description: m.netview_restart_lockout_body(),
      action: m.netview_restart_anyway(),
      tone: 'danger',
    });
    if (anyway) {
      notify.progress(m.netview_restarting({ name }), id);
      result = await restartUplink(name, true);
    }
  }
  restarting = undefined;
  if ('error' in result) notify.error(result.error, id);
  else if ('up' in result) {
    if (result.up) notify.success(m.netview_restart_up({ name }), id);
    else notify.info(m.netview_restart_not_up({ name }), id);
  }
  void ifaces.retry();
}
</script>

<Section title={m.netview_interfaces()} description={m.netview_intro()}>
  <LoadState status={ifaces.status} retry={ifaces.retry} stale={ifaces.data !== undefined}>
    {#if ifaces.data?.partial}
      <p class="mt-2" role="note"><Status kind="warn">{m.netview_partial()}</Status></p>
    {/if}
    {#if ifaces.data}
      <Table.Root aria-label={m.netview_interfaces()}>
        <Table.Header>
          <Table.Row>
            <Table.Head>{m.netview_col_interface()}</Table.Head>
            <Table.Head>{m.netview_col_status()}</Table.Head>
            <Table.Head class="max-md:hidden">{m.netview_col_protocol()}</Table.Head>
            <Table.Head class="max-sm:hidden">{m.netview_col_addresses()}</Table.Head>
            <Table.Head class="w-12"><span class="sr-only">{m.table_actions()}</span></Table.Head>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {#each ifaces.data.interfaces as it (it.name)}
            {@const state = interfaceStatus(it)}
            {@const addrs = interfaceAddresses(it)}
            <Table.Row>
              <Table.Cell class="whitespace-normal">
                <span class="font-medium">{it.name}</span>
                {#if it.zone}<span class="block text-xs text-muted-foreground">{m.netview_zone({ zone: it.zone })}</span>{/if}
              </Table.Cell>
              <Table.Cell class="whitespace-normal">
                <div class="flex flex-wrap gap-1">
                  <Status kind={state.kind}>{state.label}</Status>
                  {#if it.uplink}<Status kind="info">{m.netview_uplink()}</Status>{/if}
                </div>
              </Table.Cell>
              <Table.Cell class="font-mono text-xs max-md:hidden">{it.proto}{#if it.device}<span class="block text-muted-foreground">{it.device}</span>{/if}</Table.Cell>
              <Table.Cell class="font-mono text-xs break-all whitespace-normal max-sm:hidden">
                {addrs || '–'}
                {#if it.dns_servers.length}<span class="block font-sans text-muted-foreground">{m.netview_dns({ servers: it.dns_servers.join(', ') })}</span>{/if}
              </Table.Cell>
              <Table.Cell class="text-right">
                {#if restartable(it.name)}
                  {@const name = it.name}
                  <Button variant="ghost" size="icon" type="button" disabled={restarting !== undefined}
                    aria-label={m.netview_restart_name({ name })}
                    onclick={() => restart(name)}><RotateCw aria-hidden="true" size={16} /></Button>
                {/if}
              </Table.Cell>
            </Table.Row>
          {/each}
        </Table.Body>
      </Table.Root>
    {/if}
  </LoadState>
</Section>

<Section title={m.netview_zones()}>
  <LoadState status={zones.status} retry={zones.retry} stale={zones.data !== undefined}>
    {#if zones.data?.length === 0}
      <EmptyState icon={Shield} title={m.netview_zones_empty()} />
    {:else if zones.data}
      <Table.Root aria-label={m.netview_zones()}>
        <Table.Header>
          <Table.Row>
            <Table.Head>{m.netview_col_zone()}</Table.Head>
            <Table.Head>{m.netview_col_policy()}</Table.Head>
            <Table.Head class="max-sm:hidden">{m.netview_col_networks()}</Table.Head>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {#each zones.data as z (z.name)}
            <Table.Row>
              <Table.Cell class="whitespace-normal">
                <span class="font-medium">{z.name}</span>
                {#if z.masq}<Status kind="info">{m.netview_masq()}</Status>{/if}
              </Table.Cell>
              <Table.Cell class="font-mono text-xs whitespace-normal">{m.netview_policies({ input: z.input, output: z.output, forward: z.forward })}</Table.Cell>
              <Table.Cell class="text-sm whitespace-normal text-muted-foreground max-sm:hidden">
                {z.networks.join(', ') || '–'}
                {#if z.forwards_to.length}<span class="block">{m.netview_forwards({ zones: z.forwards_to.join(', ') })}</span>{/if}
              </Table.Cell>
            </Table.Row>
          {/each}
        </Table.Body>
      </Table.Root>
    {/if}
  </LoadState>
</Section>

<ConfirmDialog {confirm} />
