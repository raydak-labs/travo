<script lang="ts">
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import { setUsbTether } from '../api/gen';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Status from '../ui/Status.svelte';
import { transactionToast } from '../ui/transaction-toast.svelte';
import {
  boundLinkMissing,
  driverState,
  fetchUsbTether,
  installDrivers,
  type Phone,
  usbTetherRefusal,
} from '../usbtether';
import { controlPhase, createTunnelControl } from '../vpn-tunnel.svelte';

const usb = createLoader(fetchUsbTether, 5000);
// The link the next enable binds; empty lets the daemon pick the only detected one.
let chosen = '';
const control = createTunnelControl(() => usb.retry(), {
  submit: (enabled, signal) =>
    setUsbTether({ body: enabled && chosen ? { enabled, device: chosen } : { enabled }, signal }),
  refusal: usbTetherRefusal,
  confirmed: m.usbtether_confirmed,
  rolledBack: m.usbtether_rolled_back,
  rollbackFailed: m.usbtether_rollback_failed,
});
transactionToast(
  'usbtether',
  () => controlPhase(control, m.usbtether_applying()),
  (ok) => void control.settle(ok),
);
const confirm = createConfirm();
const request = new AbortController();
let installing = $state<Phone>();
onDestroy(() => {
  usb.stop();
  control.stop();
  request.abort();
});

const kinds: Record<string, () => string> = {
  rndis: m.usbtether_kind_android,
  ncm: m.usbtether_kind_android,
  ecm: m.usbtether_kind_ecm,
  ipheth: m.usbtether_kind_iphone,
};

async function install(phone: Phone) {
  if (installing) return;
  installing = phone;
  notify.progress(m.usbtether_installing(), 'usbtether-install');
  const result = await installDrivers(phone, request.signal);
  if (request.signal.aborted) return;
  installing = undefined;
  if ('error' in result) notify.error(result.error, 'usbtether-install');
  else notify.success(result.message, 'usbtether-install');
  await usb.retry();
}

// Both directions change which uplink carries traffic; disabling the only uplink cuts the router off.
async function change(enable: boolean, device = '') {
  const ok = await confirm.ask({
    title: enable ? m.usbtether_enable_title({ device }) : m.usbtether_disable_title(),
    description: enable ? m.usbtether_enable_help() : m.usbtether_disable_help(),
    action: enable ? m.usbtether_enable() : m.usbtether_disable(),
    tone: enable ? 'default' : 'danger',
  });
  if (!ok) return;
  chosen = device;
  void control.set(enable);
}
</script>

<Card title={m.usbtether_title()}>
  <LoadState status={usb.status} retry={usb.retry} stale={usb.data !== undefined}>
    {#if usb.data}
      {@const st = usb.data}
      <div class="flex flex-wrap items-center gap-2">
        {#if st.configured && st.up}
          <Status kind="ok">{m.usbtether_state_up({ device: st.device, ipv4: st.ipv4 || '—' })}</Status>
        {:else if st.configured}
          <Status kind="warn">{m.usbtether_state_down({ device: st.device })}</Status>
        {:else}
          <Status kind="neutral">{m.usbtether_state_off()}</Status>
        {/if}
        {#if st.install_blocked}<Status kind="warn">{m.usbtether_install_blocked()}</Status>{/if}
      </div>
      {#if boundLinkMissing(st)}
        <p class="text-muted-foreground">{m.usbtether_bound_missing({ device: st.device })}</p>
      {/if}

      <h3 class="font-semibold">{m.usbtether_phones()}</h3>
      {#if st.links.length === 0}
        <p class="text-muted-foreground">{m.usbtether_no_phone()}</p>
      {:else}
        <ul class="flex flex-col gap-2">
          {#each st.links as link (link.name)}
            <li class="flex flex-wrap items-center gap-2">
              <span class="font-mono">{link.name}</span>
              <span class="text-muted-foreground">{kinds[link.kind]?.() ?? link.kind}</span>
              {#if link.carrier === null}
                <Status kind="neutral">{m.dashboard_connectivity_unknown()}</Status>
              {:else}
                <Status kind={link.carrier ? 'ok' : 'danger'}>{link.carrier ? m.dashboard_carrier_up() : m.dashboard_carrier_down()}</Status>
              {/if}
              {#if !(st.configured && st.device === link.name) && !control.busy}
                <Button variant="outline" size="sm" type="button" onclick={() => change(true, link.name)}>{m.usbtether_enable()}</Button>
              {/if}
            </li>
          {/each}
        </ul>
      {/if}

      <h3 class="font-semibold">{m.usbtether_drivers()}</h3>
      <ul class="flex flex-col gap-2">
        {#each [['android', m.usbtether_drivers_android], ['ios', m.usbtether_drivers_ios]] as const as [phone, label] (phone)}
          {@const ds = driverState(st, phone)}
          <li class="flex flex-wrap items-center gap-2">
            <span>{label()}</span>
            <Status kind={ds === 'installed' ? 'ok' : ds === 'unknown' ? 'neutral' : 'warn'}>
              {ds === 'installed' ? m.usbtether_driver_installed() : ds === 'unknown' ? m.dashboard_connectivity_unknown() : m.usbtether_driver_absent()}
            </Status>
            {#if ds !== 'installed'}
              <Button variant="outline" size="sm" type="button" disabled={installing !== undefined} onclick={() => install(phone)}>
                {installing === phone ? m.usbtether_installing() : m.usbtether_install()}
              </Button>
            {/if}
          </li>
        {/each}
      </ul>
      <p class="text-muted-foreground">{m.usbtether_drivers_help()}</p>
      {#if st.configured && !control.busy}
        <div class="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" type="button" onclick={() => change(false)}>{m.usbtether_disable()}</Button>
        </div>
      {/if}
    {/if}
  </LoadState>
</Card>

<ConfirmDialog {confirm} />
