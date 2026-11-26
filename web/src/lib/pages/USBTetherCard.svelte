<script lang="ts">
import Smartphone from '@lucide/svelte/icons/smartphone';
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import { setUsbTether } from '../api/gen';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import GroupHeading from '../ui/GroupHeading.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import Section from '../ui/Section.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import Status from '../ui/Status.svelte';
import { transactionToast } from '../ui/transaction-toast.svelte';
import {
  boundLinkMissing,
  driverState,
  fetchUsbTether,
  installDrivers,
  type Phone,
  usbTetherRefusal,
  withIphoneHint,
} from '../usbtether';
import { controlPhase, createTunnelControl } from '../vpn-tunnel.svelte';

const usb = createLoader(fetchUsbTether, 5000);
// The link the next enable binds; empty lets the daemon pick the only detected one.
let chosen = '';
const txn = createTunnelControl(() => usb.retry(), {
  submit: (enabled, signal) =>
    setUsbTether({ body: enabled && chosen ? { enabled, device: chosen } : { enabled }, signal }),
  refusal: (code) => {
    const r = usbTetherRefusal(code);
    return r.error ? { error: withIphoneHint(r.error, usb.data, chosen) } : r;
  },
  confirmed: m.usbtether_confirmed,
  rolledBack: () => withIphoneHint(m.usbtether_rolled_back(), usb.data, chosen),
  rollbackFailed: m.usbtether_rollback_failed,
});
transactionToast(
  'usbtether',
  () => controlPhase(txn, m.usbtether_applying()),
  (ok) => void txn.settle(ok),
);
const confirm = createConfirm();
const request = new AbortController();
let installing = $state<Phone>();
onDestroy(() => {
  usb.stop();
  txn.stop();
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
// An iPhone only carries traffic once it is unlocked, trusts the router and shares its hotspot.
async function change(enable: boolean, device = '', kind = '') {
  const help =
    kind === 'ipheth'
      ? `${m.usbtether_enable_help()} ${m.usbtether_iphone_hint()}`
      : m.usbtether_enable_help();
  const ok = await confirm.ask({
    title: enable ? m.usbtether_enable_title({ device }) : m.usbtether_disable_title(),
    description: enable ? help : m.usbtether_disable_help(),
    action: enable ? m.usbtether_enable() : m.usbtether_disable(),
    tone: enable ? 'default' : 'danger',
  });
  if (!ok) return;
  chosen = device;
  void txn.set(enable);
}
</script>

<Section title={m.usbtether_title()}>
  {#snippet actions()}
    {#if usb.data}
      {@const st = usb.data}
      {#if st.install_blocked}<Status kind="warn">{m.usbtether_install_blocked()}</Status>{/if}
      {#if st.configured && st.up}
        <Status kind="ok">{m.usbtether_state_up({ device: st.device, ipv4: st.ipv4 || '—' })}</Status>
      {:else if st.configured}
        <Status kind="warn">{m.usbtether_state_down({ device: st.device })}</Status>
      {:else}
        <Status kind="neutral">{m.usbtether_state_off()}</Status>
      {/if}
    {/if}
  {/snippet}
  <LoadState status={usb.status} retry={usb.retry} stale={usb.data !== undefined}>
    {#if usb.data}
      {@const st = usb.data}
      {#if boundLinkMissing(st)}
        <p class="text-muted-foreground">{m.usbtether_bound_missing({ device: st.device })}</p>
      {/if}

      <GroupHeading label={m.usbtether_phones()} />
      {#if st.links.length === 0}
        <div class="flex items-start gap-3 py-2">
          <Smartphone aria-hidden="true" size={18} class="mt-0.5 shrink-0 text-muted-foreground" />
          <div>
            <p class="text-sm font-medium">{m.usbtether_no_phone_title()}</p>
            <p class="text-sm text-muted-foreground">{m.usbtether_no_phone()}</p>
          </div>
        </div>
      {:else}
        <div>
          {#each st.links as link (link.name)}
            <SettingsRow label={link.name} description={link.kind === 'ipheth' && link.carrier === false ? `${kinds[link.kind]?.() ?? link.kind} · ${m.usbtether_iphone_hint()}` : (kinds[link.kind]?.() ?? link.kind)}>
              {#snippet control()}
              {#if link.carrier === null}
                <Status kind="neutral">{m.dashboard_connectivity_unknown()}</Status>
              {:else}
                <Status kind={link.carrier ? 'ok' : 'danger'}>{link.carrier ? m.dashboard_carrier_up() : m.dashboard_carrier_down()}</Status>
              {/if}
              {#if !(st.configured && st.device === link.name) && !txn.busy}
                <Button variant="outline" size="sm" type="button" onclick={() => change(true, link.name, link.kind)}>{m.usbtether_enable()}</Button>
              {/if}
              {/snippet}
            </SettingsRow>
          {/each}
        </div>
      {/if}

      <GroupHeading label={m.usbtether_drivers()} />
      <div>
        {#each [['android', m.usbtether_drivers_android], ['ios', m.usbtether_drivers_ios]] as const as [phone, label] (phone)}
          {@const ds = driverState(st, phone)}
          <SettingsRow label={label()}>
            {#snippet control()}
            <Status kind={ds === 'installed' ? 'ok' : ds === 'unknown' ? 'neutral' : 'warn'}>
              {ds === 'installed' ? m.usbtether_driver_installed() : ds === 'unknown' ? m.dashboard_connectivity_unknown() : m.usbtether_driver_absent()}
            </Status>
            {#if ds !== 'installed'}
              <Button variant="outline" size="sm" type="button" disabled={installing !== undefined} onclick={() => install(phone)}>
                {installing === phone ? m.usbtether_installing() : m.usbtether_install()}
              </Button>
            {/if}
            {/snippet}
          </SettingsRow>
        {/each}
      </div>
      <p class="text-sm text-muted-foreground">{m.usbtether_drivers_help()}</p>
      {#if st.configured && !txn.busy}
        <div class="flex flex-wrap gap-2">
          <Button variant="destructive" size="sm" type="button" onclick={() => change(false)}>{m.usbtether_disable()}</Button>
        </div>
      {/if}
    {/if}
  </LoadState>
</Section>

<ConfirmDialog {confirm} />
