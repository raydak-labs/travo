<script lang="ts">
import Shield from '@lucide/svelte/icons/shield';
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
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
      <ul class="divide-y divide-border" aria-label={m.netview_interfaces()}>
        {#each ifaces.data.interfaces as it (it.name)}
          {@const state = interfaceStatus(it)}
          {@const addrs = interfaceAddresses(it)}
          <li class="flex flex-col gap-1 py-3">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-medium">{it.name}</span>
              <Status kind={state.kind}>{state.label}</Status>
              {#if it.uplink}<Status kind="info">{m.netview_uplink()}</Status>{/if}
              {#if it.zone}<span class="text-sm text-muted-foreground">{m.netview_zone({ zone: it.zone })}</span>{/if}
              {#if restartable(it.name)}
                {@const name = it.name}
                <Button variant="outline" size="sm" type="button" class="ml-auto" disabled={restarting !== undefined}
                  aria-label={m.netview_restart_name({ name })}
                  onclick={() => restart(name)}>{m.netview_restart()}</Button>
              {/if}
            </div>
            <p class="break-all font-mono text-sm text-muted-foreground">
              {it.proto}{#if it.device} · {it.device}{/if}{#if addrs} · {addrs}{/if}
            </p>
            {#if it.dns_servers.length}
              <p class="break-all text-sm text-muted-foreground">{m.netview_dns({ servers: it.dns_servers.join(', ') })}</p>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
  </LoadState>
</Section>

<Section title={m.netview_zones()}>
  <LoadState status={zones.status} retry={zones.retry} stale={zones.data !== undefined}>
    {#if zones.data?.length === 0}
      <EmptyState icon={Shield} title={m.netview_zones_empty()} />
    {:else if zones.data}
      <ul class="divide-y divide-border" aria-label={m.netview_zones()}>
        {#each zones.data as z (z.name)}
          <li class="flex flex-col gap-1 py-3">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-medium">{z.name}</span>
              {#if z.masq}<Status kind="info">{m.netview_masq()}</Status>{/if}
            </div>
            <p class="font-mono text-sm text-muted-foreground">
              {m.netview_policies({ input: z.input, output: z.output, forward: z.forward })}
            </p>
            <p class="break-all text-sm text-muted-foreground">
              {m.netview_networks({ networks: z.networks.join(', ') || '–' })}
              {#if z.forwards_to.length} · {m.netview_forwards({ zones: z.forwards_to.join(', ') })}{/if}
            </p>
          </li>
        {/each}
      </ul>
    {/if}
  </LoadState>
</Section>

<ConfirmDialog {confirm} />
