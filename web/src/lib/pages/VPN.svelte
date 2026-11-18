<script lang="ts">
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import Input from '$lib/components/ui/input/input.svelte';
import Label from '$lib/components/ui/label/label.svelte';
import Textarea from '$lib/components/ui/textarea/textarea.svelte';
import '../api/client';
import { deleteVpnProfile, getVpnProfiles, postVpnProfile, selectVpnProfile } from '../api/gen';
import { fetchVpnStatus } from '../api/vpn-status';
import { m } from '../paraglide/messages.js';
import { href } from '../router';
import Card from '../ui/Card.svelte';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import { notify } from '../ui/notify';
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

<div class="grid gap-4">
  <Card title={m.vpn_tunnel()}>
    <VPNTunnelPanel {status} control={tunnel} canEnable={() => Boolean(profiles.data?.selected_id)}>
      {#snippet extra(snapshot)}
        {#if tunnelEnabled(snapshot.state) && profiles.data?.selected_id}
          <Button variant="outline" size="sm" type="button" disabled={snapshot.stale} onclick={() => tunnel.set(true)}>{m.vpn_tunnel_apply()}</Button>
        {/if}
      {/snippet}
    </VPNTunnelPanel>
  </Card>

  <Card title={m.tailscale_title()}>
    <p class="text-muted-foreground">{m.tailscale_vpn_intro()}</p>
    <Button variant="outline" size="sm" class="self-start" href={href('vpn-tailscale')}>{m.tailscale_open()}</Button>
  </Card>

  <VPNKillSwitchCard tunnel={status.data} />

  <VPNSplitCard />
  <VPNDNSCard />

  <Card title={m.vpn_saved_profiles()}>
    <LoadState status={profiles.status} retry={profiles.retry}>
      {#if profiles.data}
        {#if profiles.data.profiles.length === 0}
          <p class="text-muted-foreground">{m.vpn_no_profiles()}</p>
        {:else}
          <ul class="space-y-3">
            {#each profiles.data.profiles as profile (profile.id)}
              <li class="flex flex-wrap items-center justify-between gap-2">
                <span class="break-all">{profile.name}</span>
                <div class="flex flex-wrap items-center gap-2">
                  {#if profile.id === profiles.data.selected_id}
                    <span class="text-muted-foreground">{m.vpn_selected()}</span>
                  {:else}
                    <Button variant="outline" size="sm" type="button" class="text-sm" disabled={pending} onclick={() => change('select', profile.id, profile.name)}>{m.vpn_select()}</Button>
                  {/if}
                  <Button variant="outline" size="sm" type="button" class="text-sm" disabled={pending} onclick={() => remove(profile)}>{m.vpn_delete()}</Button>
                </div>
              </li>
            {/each}
          </ul>
        {/if}
      {/if}
    </LoadState>
  </Card>

  <Card title={m.vpn_import()}>
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
  </Card>
</div>

<ConfirmDialog {confirm} />
