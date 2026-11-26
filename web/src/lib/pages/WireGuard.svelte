<script lang="ts">
import EllipsisVertical from '@lucide/svelte/icons/ellipsis-vertical';
import FileUp from '@lucide/svelte/icons/file-up';
import KeyRound from '@lucide/svelte/icons/key-round';
import Plus from '@lucide/svelte/icons/plus';
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import * as Dialog from '$lib/components/ui/dialog';
import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import * as Table from '$lib/components/ui/table';
import Textarea from '$lib/components/ui/textarea/textarea.svelte';
import '../api/client';
import { deleteVpnProfile, getVpnProfiles, postVpnProfile, selectVpnProfile } from '../api/gen';
import { fetchVpnStatus } from '../api/vpn-status';
import { formatUptime } from '../format';
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
import { createTunnelControl, tunnelStates } from '../vpn-tunnel.svelte';
import VPNTunnelSwitch from './VPNTunnelSwitch.svelte';

// WireGuard: the tunnel switch and the profile list. Adding a profile is a dialog (paste or
// file); using or deleting one is in the row menu.
const request = new AbortController();
const status = createLoader(fetchVpnStatus, 10000);
const profiles = createLoader(async (signal) => {
  const result = await getVpnProfiles({ signal });
  if (!result.data) throw new Error(m.vpn_unavailable());
  return result.data;
});
const tunnel = createTunnelControl(() => status.retry());
const confirm = createConfirm();
const formId = $props.id();
// Why the switch is off-limits, or how fresh the tunnel is; shown under the header.
const tunnelHint = $derived.by(() => {
  const snapshot = status.data;
  if (!snapshot) return undefined;
  if (!profiles.data?.selected_id) return m.vpn_tunnel_needs_profile();
  return snapshot.handshake_age_seconds !== null
    ? m.vpn_handshake_age({ age: formatUptime(snapshot.handshake_age_seconds) })
    : undefined;
});
let adding = $state(false);
let name = $state('');
let config = $state('');
let pending = $state(false);
onDestroy(() => {
  status.stop();
  profiles.stop();
  tunnel.stop();
  request.abort();
  config = '';
});

function openAdd() {
  name = '';
  config = '';
  adding = true;
}

async function readFile(event: Event & { currentTarget: HTMLInputElement }) {
  const file = event.currentTarget.files?.[0];
  if (!file) return;
  config = await file.text();
  if (!name.trim()) name = file.name.replace(/\.conf$/i, '');
}

async function remove(profile: { id: string; name: string }) {
  const ok = await confirm.ask({
    title: m.vpn_delete_title({ name: profile.name }),
    description: m.vpn_delete_body(),
    action: m.vpn_delete(),
    tone: 'danger',
  });
  if (ok) void change('delete', profile.id, profile.name);
}

async function change(action: 'import' | 'select' | 'delete', id = '', label = name) {
  if (pending) return;
  pending = true;
  try {
    const options = { signal: request.signal };
    const response =
      action === 'import'
        ? await postVpnProfile({ ...options, body: { name, config } })
        : action === 'select'
          ? await selectVpnProfile({ ...options, path: { id } })
          : await deleteVpnProfile({ ...options, path: { id } });
    if (request.signal.aborted) return;
    if (!response.data) {
      const code = (response.error as { code?: string } | undefined)?.code;
      notify.error(
        m.vpn_change_failed(),
        undefined,
        code === 'invalid_profile'
          ? m.vpn_invalid()
          : code === 'profile_limit'
            ? m.vpn_limit()
            : m.vpn_save_failed(),
      );
      return;
    }
    if (action === 'import') {
      adding = false;
      name = '';
      config = '';
    }
    notify.success(
      action === 'import'
        ? m.vpn_imported({ name: label })
        : action === 'select'
          ? m.vpn_profile_selected({ name: label })
          : m.vpn_profile_deleted({ name: label }),
    );
    await profiles.retry();
  } catch {
    if (!request.signal.aborted) notify.error(m.vpn_save_failed());
  } finally {
    if (!request.signal.aborted) pending = false;
  }
}
</script>

<div class="flex flex-col gap-6">
  <PageHeader title={m.nav_vpn_wireguard()} description={m.wireguard_page_description()}>
    {#snippet actions()}
      {#if status.data}
        {@const snapshot = status.data}
        {@const badge = tunnelStates[snapshot.state]}
        <span title={tunnelHint ?? ''}><Status kind={badge.kind}>{badge.label()}</Status></span>
        {#if snapshot.stale}<Status kind="stale">{m.load_stale()}</Status>{/if}
        <VPNTunnelSwitch id="wireguard-tunnel-switch" {snapshot} control={tunnel} canEnable={Boolean(profiles.data?.selected_id)} />
      {/if}
    {/snippet}
  </PageHeader>
  {#if tunnelHint}<p class="-mt-4 text-sm text-muted-foreground">{tunnelHint}</p>{/if}
  {#if status.status.state !== 'ready' && !status.data}
    <LoadState status={status.status} retry={status.retry}><span></span></LoadState>
  {/if}

  <Section title={m.vpn_saved_profiles()}>
    {#snippet actions()}
      <Button variant="outline" size="sm" type="button" disabled={pending} onclick={openAdd}><Plus aria-hidden="true" />{m.vpn_add_profile()}</Button>
    {/snippet}
    <LoadState status={profiles.status} retry={profiles.retry} stale={profiles.data !== undefined}>
      {#if profiles.data}
        {#if profiles.data.profiles.length === 0}
          <EmptyState icon={KeyRound} title={m.vpn_no_profiles()} />
        {:else}
          {@const selectedId = profiles.data.selected_id}
          <Table.Root>
            <Table.Header>
              <Table.Row>
                <Table.Head>{m.vpn_name()}</Table.Head>
                <Table.Head>{m.vpn_status_label()}</Table.Head>
                <Table.Head class="w-12"><span class="sr-only">{m.table_actions()}</span></Table.Head>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {#each profiles.data.profiles as profile (profile.id)}
                <Table.Row>
                  <Table.Cell class="font-medium">{profile.name}</Table.Cell>
                  <Table.Cell>
                    {#if profile.id === selectedId}<Status kind="info">{m.vpn_selected()}</Status>{/if}
                  </Table.Cell>
                  <Table.Cell class="text-right">
                    <DropdownMenu.Root>
                      <DropdownMenu.Trigger>
                        {#snippet child({ props })}
                          <Button {...props} variant="ghost" size="icon" aria-label={m.table_row_menu({ name: profile.name })} disabled={pending}>
                            <EllipsisVertical aria-hidden="true" size={16} />
                          </Button>
                        {/snippet}
                      </DropdownMenu.Trigger>
                      <DropdownMenu.Content align="end">
                        <DropdownMenu.Item disabled={profile.id === selectedId} onSelect={() => change('select', profile.id, profile.name)}>{m.vpn_select()}</DropdownMenu.Item>
                        <DropdownMenu.Separator />
                        <DropdownMenu.Item variant="destructive" onSelect={() => remove(profile)}>{m.vpn_delete()}</DropdownMenu.Item>
                      </DropdownMenu.Content>
                    </DropdownMenu.Root>
                  </Table.Cell>
                </Table.Row>
              {/each}
            </Table.Body>
          </Table.Root>
        {/if}
      {/if}
    </LoadState>
  </Section>
</div>

<Dialog.Root bind:open={adding} onOpenChange={(open) => { if (!open) config = ''; }}>
  <Dialog.Content portalProps={{ disabled: typeof window === 'undefined' }}>
    <Dialog.Header>
      <Dialog.Title level={2}>{m.vpn_add_profile()}</Dialog.Title>
      <Dialog.Description>{m.vpn_import_description()}</Dialog.Description>
    </Dialog.Header>
    <form id={formId} class="grid gap-4" autocomplete="off" onsubmit={(event) => { event.preventDefault(); void change('import'); }}>
      <div class="grid gap-2">
        <Label for="{formId}-name">{m.vpn_name()}</Label>
        <Input id="{formId}-name" bind:value={name} required disabled={pending} />
      </div>
      <div class="grid gap-2">
        <Label for="{formId}-config">{m.vpn_config()}</Label>
        <Textarea id="{formId}-config" class="w-full font-mono text-xs" rows={8} bind:value={config} required disabled={pending} spellcheck="false" placeholder="[Interface]"></Textarea>
      </div>
      <div>
        <input id="{formId}-file" type="file" accept=".conf,text/plain" class="sr-only" disabled={pending} onchange={readFile} />
        <Button variant="outline" size="sm" type="button" disabled={pending}
          onclick={() => document.getElementById(`${formId}-file`)?.click()}><FileUp aria-hidden="true" />{m.vpn_config_file()}</Button>
      </div>
    </form>
    <Dialog.Footer>
      <Button variant="outline" type="button" onclick={() => { adding = false; }}>{m.dialog_cancel()}</Button>
      <Button type="submit" form={formId} disabled={pending || !name.trim() || !config.trim()}>{m.vpn_add_profile()}</Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

<ConfirmDialog {confirm} />
