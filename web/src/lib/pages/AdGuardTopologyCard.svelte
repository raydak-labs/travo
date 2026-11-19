<script lang="ts">
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import * as Collapsible from '$lib/components/ui/collapsible';
import { topologyRefusal } from '../api/adguard';
import { setAdGuardTopology } from '../api/gen';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
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

const control = createTunnelControl(() => refresh(), {
  submit: (on, signal) => setAdGuardTopology({ body: { mode: on ? 'expert' : 'default' }, signal }),
  refusal: topologyRefusal,
  confirmed: m.adguard_topology_confirmed,
  rolledBack: m.adguard_topology_rolled_back,
  rollbackFailed: m.adguard_topology_rollback_failed,
  failed: m.adguard_err_failed,
});
transactionToast(
  'adguard-topology',
  () => controlPhase(control, m.adguard_topology_applying()),
  (ok) => void control.settle(ok),
);
const confirm = createConfirm();
onDestroy(() => control.stop());

async function toggle(on: boolean) {
  if (control.busy) return;
  const ok = await confirm.ask({
    title: on ? m.adguard_topology_confirm_on_title() : m.adguard_topology_confirm_off_title(),
    description: on ? m.adguard_topology_confirm_on() : m.adguard_topology_confirm_off(),
    action: on ? m.adguard_topology_turn_on() : m.adguard_topology_turn_off(),
    tone: on ? 'danger' : 'default',
  });
  if (ok) void control.set(on);
}
</script>

<Card title={m.adguard_topology_title()}>
  <Collapsible.Root open={expert}>
    <Collapsible.Trigger class="cursor-pointer font-medium">{m.adguard_topology_disclosure()}</Collapsible.Trigger>
    <Collapsible.Content>
      <p class="mt-2 text-muted-foreground">{m.adguard_topology_intro()}</p>
      <div class="mt-3 flex flex-wrap items-center gap-3">
        <Status kind={expert ? 'warn' : 'neutral'}>{expert ? m.adguard_topology_state_expert() : m.adguard_topology_state_default()}</Status>
        <Button variant="outline" type="button" disabled={control.busy || busy} onclick={() => toggle(!expert)}>
          {control.applying ? m.adguard_topology_applying() : expert ? m.adguard_topology_turn_off() : m.adguard_topology_turn_on()}
        </Button>
      </div>
      {#if expert && hosts.length > 0}
        <p class="mt-2 text-muted">{m.adguard_topology_hosts({ hosts: hosts.join(', ') })}</p>
      {/if}
    </Collapsible.Content>
  </Collapsible.Root>
</Card>

<ConfirmDialog {confirm} />
