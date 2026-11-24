<script lang="ts">
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import Textarea from '$lib/components/ui/textarea/textarea.svelte';
import '../api/client';
import KeyRound from '@lucide/svelte/icons/key-round';
import { deleteVpnProfile, getVpnProfiles, postVpnProfile, selectVpnProfile } from '../api/gen';
import { fetchVpnStatus } from '../api/vpn-status';
import { m } from '../paraglide/messages.js';
import { href } from '../router';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import EmptyState from '../ui/EmptyState.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
import PageHeader from '../ui/PageHeader.svelte';
import Section from '../ui/Section.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import { createTunnelControl, tunnelEnabled } from '../vpn-tunnel.svelte';
import VPNDNSCard from './VPNDNSCard.svelte';
import VPNKillSwitchCard from './VPNKillSwitchCard.svelte';
import VPNSplitCard from './VPNSplitCard.svelte';
import VPNTunnelPanel from './VPNTunnelPanel.svelte';

const request = new AbortController();
const status = createLoader(fetchVpnStatus, 10000);
const profiles = createLoader(async (signal) => {
  const result = await getVpnProfiles({ signal });
  if (!result.data) throw new Error(m.vpn_unavailable());
  return result.data;
});
let name = $state('');
let config = $state('');
let pending = $state(false);
const confirm = createConfirm();
const tunnel = createTunnelControl(() => status.retry());
onDestroy(() => {
  status.stop();
  tunnel.stop();
  profiles.stop();
  request.abort();
  config = '';
});

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
  <PageHeader title={m.nav_vpn()} description={m.vpn_page_description()} />

  <Section title={m.vpn_tunnel()}>
    <VPNTunnelPanel {status} control={tunnel} canEnable={() => Boolean(profiles.data?.selected_id)}>
      {#snippet extra(snapshot)}
        {#if tunnelEnabled(snapshot.state) && profiles.data?.selected_id}
          <Button variant="outline" size="sm" type="button" disabled={snapshot.stale} onclick={() => tunnel.set(true)}>{m.vpn_tunnel_apply()}</Button>
        {/if}
      {/snippet}
    </VPNTunnelPanel>
  </Section>

  <Section title={m.tailscale_title()} description={m.tailscale_vpn_intro()}>
    {#snippet actions()}
      <Button variant="outline" size="sm" href={href('vpn-tailscale')}>{m.tailscale_open()}</Button>
    {/snippet}
  </Section>

  <VPNKillSwitchCard tunnel={status.data} />

  <VPNSplitCard />
  <VPNDNSCard />

  <Section title={m.vpn_saved_profiles()}>
    <LoadState status={profiles.status} retry={profiles.retry}>
      {#if profiles.data}
        {#if profiles.data.profiles.length === 0}
          <EmptyState icon={KeyRound} title={m.vpn_no_profiles()} />
        {:else}
          {@const selectedId = profiles.data.selected_id}
          <div>
            {#each profiles.data.profiles as profile (profile.id)}
              <SettingsRow label={profile.name}>
                {#snippet control()}
                  {#if profile.id === selectedId}
                    <span class="text-muted-foreground">{m.vpn_selected()}</span>
                  {:else}
                    <Button variant="outline" size="sm" type="button" disabled={pending} onclick={() => change('select', profile.id, profile.name)}>{m.vpn_select()}</Button>
                  {/if}
                  <Button variant="destructive" size="sm" type="button" disabled={pending} onclick={() => remove(profile)}>{m.vpn_delete()}</Button>
                {/snippet}
              </SettingsRow>
            {/each}
          </div>
        {/if}
      {/if}
    </LoadState>
  </Section>

  <Section title={m.vpn_import()} description={m.vpn_import_description()}>
    <form class="grid gap-3" autocomplete="off" onsubmit={(event) => { event.preventDefault(); void change('import'); }}>
      <Label class="grid gap-1">
        <span>{m.vpn_name()}</span>
        <Input  bind:value={name} required disabled={pending} />
      </Label>
      <Label class="grid gap-1">
        <span>{m.vpn_config()}</span>
        <Textarea class="w-full font-mono" rows={6} bind:value={config} required disabled={pending} spellcheck="false"></Textarea>
      </Label>
      <Button variant="default" type="submit" class="w-fit" disabled={pending || !name.trim() || !config.trim()}>{m.vpn_import()}</Button>
    </form>
  </Section>
</div>

<ConfirmDialog {confirm} />
