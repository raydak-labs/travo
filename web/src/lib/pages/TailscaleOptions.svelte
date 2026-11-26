<script lang="ts">
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import '../api/client';
import Globe from '@lucide/svelte/icons/globe';
import { applyTailscaleFirewall, type TailscaleSettings, type TailscaleStatus } from '../api/gen';
import { firewallRefusal } from '../api/tailscale';
import { m } from '../paraglide/messages.js';
import type { createConfirm } from '../ui/confirm.svelte';
import EmptyState from '../ui/EmptyState.svelte';
import Section from '../ui/Section.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import Status from '../ui/Status.svelte';
import { transactionToast } from '../ui/transaction-toast.svelte';
import { controlPhase, createTunnelControl } from '../vpn-tunnel.svelte';

let {
  status,
  disabled,
  confirm,
  onchange,
  refresh,
}: {
  status: TailscaleStatus;
  disabled: boolean;
  confirm: ReturnType<typeof createConfirm>;
  onchange: (settings: TailscaleSettings) => void;
  refresh: () => unknown;
} = $props();

const firewall = createTunnelControl(() => refresh(), {
  submit: (_enabled, signal) => applyTailscaleFirewall({ signal }),
  refusal: firewallRefusal,
  confirmed: m.tailscale_fw_confirmed,
  rolledBack: m.tailscale_fw_rolled_back,
  rollbackFailed: m.tailscale_fw_rollback_failed,
});
transactionToast(
  'tailscale-firewall',
  () => controlPhase(firewall, m.tailscale_applying()),
  (ok) => void firewall.settle(ok),
);
onDestroy(() => firewall.stop());

const settings = $derived(status.settings);
const exitNodes = $derived(status.exit_nodes ?? []);

/** Every change sends the complete settings value (strict PUT). */
function save(patch: Partial<TailscaleSettings>) {
  onchange({ ...settings, ...patch });
}

// An exit node moves the default route of the router and every LAN client.
async function useExitNode(id: string, name: string) {
  const ok = await confirm.ask({
    title: m.tailscale_exit_use_title({ name }),
    description: m.tailscale_exit_use_body(),
    action: m.tailscale_exit_use(),
  });
  if (ok) save({ exit_node: id });
}
</script>

<Section title={m.tailscale_exit_title()} description={m.tailscale_exit_help()}>
  {#if exitNodes.length === 0}
    <EmptyState icon={Globe} title={m.tailscale_exit_none()} />
  {:else}
    <div>
      {#each exitNodes as node (node.id)}
        <SettingsRow label={node.host_name}>
          {#snippet control()}
          <Status kind={node.online ? 'ok' : 'neutral'}>{node.online ? m.tailscale_online() : m.tailscale_offline()}</Status>
          {#if node.id === settings.exit_node}
            <Button variant="outline" size="sm" type="button" {disabled} onclick={() => save({ exit_node: '' })}>{m.tailscale_exit_stop()}</Button>
          {:else}
            <Button variant="outline" size="sm" type="button" disabled={disabled || !node.online} onclick={() => useExitNode(node.id, node.host_name)}>{m.tailscale_exit_use()}</Button>
          {/if}
          {/snippet}
        </SettingsRow>
      {/each}
    </div>
  {/if}
</Section>

<Section title={m.tailscale_options_title()}>
  <div>
    <SettingsRow label={m.tailscale_advertise_lan()} description={m.tailscale_advertise_lan_help()}>
      {#snippet control()}
      <Button variant="outline" size="sm" type="button" {disabled} onclick={() => save({ advertise_lan: !settings.advertise_lan })}>{settings.advertise_lan ? m.tailscale_turn_off() : m.tailscale_turn_on()}</Button>
      {/snippet}
    </SettingsRow>
    <SettingsRow label={m.tailscale_ssh()} description={m.tailscale_ssh_help()}>
      {#snippet control()}
      <Button variant="outline" size="sm" type="button" {disabled} onclick={() => save({ ssh: !settings.ssh })}>{settings.ssh ? m.tailscale_turn_off() : m.tailscale_turn_on()}</Button>
      {/snippet}
    </SettingsRow>
  </div>
  <div class="flex flex-wrap items-center gap-2">
    <Status kind={status.firewall_in_sync ? 'ok' : 'warn'}>{status.firewall_in_sync ? m.tailscale_fw_ok() : m.tailscale_fw_needed()}</Status>
  </div>
  {#if !status.firewall_in_sync}
    <p class="text-muted-foreground">{m.tailscale_fw_help()}</p>
    {#if !firewall.busy}
      <Button variant="outline" size="sm" type="button" class="self-start" {disabled} onclick={() => firewall.set(true)}>{m.tailscale_fw_apply()}</Button>
    {/if}
  {/if}
</Section>
