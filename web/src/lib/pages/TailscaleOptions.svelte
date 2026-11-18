<script lang="ts">
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import '../api/client';
import { applyTailscaleFirewall, type TailscaleSettings, type TailscaleStatus } from '../api/gen';
import { firewallRefusal } from '../api/tailscale';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import type { createConfirm } from '../ui/confirm.svelte';
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

<Card title={m.tailscale_exit_title()}>
  <p class="text-muted-foreground">{m.tailscale_exit_help()}</p>
  {#if exitNodes.length === 0}
    <p class="text-muted-foreground">{m.tailscale_exit_none()}</p>
  {:else}
    <ul class="space-y-3">
      {#each exitNodes as node (node.id)}
        <li class="flex flex-wrap items-center justify-between gap-2">
          <span class="flex flex-wrap items-center gap-2">
            <span class="break-all">{node.host_name}</span>
            <Status kind={node.online ? 'ok' : 'neutral'}>{node.online ? m.tailscale_online() : m.tailscale_offline()}</Status>
          </span>
          {#if node.id === settings.exit_node}
            <Button variant="outline" size="sm" type="button" {disabled} onclick={() => save({ exit_node: '' })}>{m.tailscale_exit_stop()}</Button>
          {:else}
            <Button variant="outline" size="sm" type="button" disabled={disabled || !node.online} onclick={() => useExitNode(node.id, node.host_name)}>{m.tailscale_exit_use()}</Button>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</Card>

<Card title={m.tailscale_options_title()}>
  <dl class="grid grid-cols-[1fr_auto] items-center gap-x-4 gap-y-3">
    <dt>
      <span class="block">{m.tailscale_advertise_lan()}</span>
      <span class="block text-muted-foreground">{m.tailscale_advertise_lan_help()}</span>
    </dt>
    <dd>
      <Button variant="outline" size="sm" type="button" {disabled} onclick={() => save({ advertise_lan: !settings.advertise_lan })}>{settings.advertise_lan ? m.tailscale_turn_off() : m.tailscale_turn_on()}</Button>
    </dd>
    <dt>
      <span class="block">{m.tailscale_ssh()}</span>
      <span class="block text-muted-foreground">{m.tailscale_ssh_help()}</span>
    </dt>
    <dd>
      <Button variant="outline" size="sm" type="button" {disabled} onclick={() => save({ ssh: !settings.ssh })}>{settings.ssh ? m.tailscale_turn_off() : m.tailscale_turn_on()}</Button>
    </dd>
  </dl>
  <div class="flex flex-wrap items-center gap-2">
    <Status kind={status.firewall_in_sync ? 'ok' : 'warn'}>{status.firewall_in_sync ? m.tailscale_fw_ok() : m.tailscale_fw_needed()}</Status>
  </div>
  {#if !status.firewall_in_sync}
    <p class="text-muted-foreground">{m.tailscale_fw_help()}</p>
    {#if !firewall.busy}
      <Button variant="outline" size="sm" type="button" class="self-start" {disabled} onclick={() => firewall.set(true)}>{m.tailscale_fw_apply()}</Button>
    {/if}
  {/if}
</Card>
