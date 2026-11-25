<script lang="ts">
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import { topologyRefusal } from '../api/adguard';
import { setAdGuardTopology } from '../api/gen';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import Section from '../ui/Section.svelte';
import Status from '../ui/Status.svelte';
import { transactionToast } from '../ui/transaction-toast.svelte';
import { controlPhase, createTunnelControl } from '../vpn-tunnel.svelte';

// ADR 0021: AdGuard Home on port 53 in front of dnsmasq. Lazy chunk of the Services page.
let {
  expert,
  hosts = [],
  busy = false,
  refresh,
}: { expert: boolean; hosts?: string[]; busy?: boolean; refresh: () => unknown } = $props();

const txn = createTunnelControl(() => refresh(), {
  submit: (on, signal) => setAdGuardTopology({ body: { mode: on ? 'expert' : 'default' }, signal }),
  refusal: topologyRefusal,
  confirmed: m.adguard_topology_confirmed,
  rolledBack: m.adguard_topology_rolled_back,
  rollbackFailed: m.adguard_topology_rollback_failed,
  failed: m.adguard_err_failed,
});
transactionToast(
  'adguard-topology',
  () => controlPhase(txn, m.adguard_topology_applying()),
  (ok) => void txn.settle(ok),
);
const confirm = createConfirm();
onDestroy(() => txn.stop());

async function toggle(on: boolean) {
  if (txn.busy) return;
  const ok = await confirm.ask({
    title: on ? m.adguard_topology_confirm_on_title() : m.adguard_topology_confirm_off_title(),
    description: on ? m.adguard_topology_confirm_on() : m.adguard_topology_confirm_off(),
    action: on ? m.adguard_topology_turn_on() : m.adguard_topology_turn_off(),
    tone: on ? 'danger' : 'default',
  });
  if (ok) void txn.set(on);
}
</script>

<Section title={m.adguard_topology_title()} description={m.adguard_topology_intro()}>
  {#snippet actions()}
    <Status kind={expert ? 'warn' : 'neutral'}>{expert ? m.adguard_topology_state_expert() : m.adguard_topology_state_default()}</Status>
  {/snippet}
  {#snippet footer()}
    <Button variant="outline" type="button" disabled={txn.busy || busy} onclick={() => toggle(!expert)}>
      {txn.applying ? m.adguard_topology_applying() : expert ? m.adguard_topology_turn_off() : m.adguard_topology_turn_on()}
    </Button>
  {/snippet}
  {#if expert && hosts.length > 0}
    <p class="text-sm text-muted-foreground">{m.adguard_topology_hosts({ hosts: hosts.join(', ') })}</p>
  {/if}
</Section>

<ConfirmDialog {confirm} />
