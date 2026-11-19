<script lang="ts">
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import { setIpv6Settings } from '../api/gen';
import { fetchIPv6, ipv6Refusal } from '../api/ipv6';
import { m } from '../paraglide/messages.js';
import Card from '../ui/Card.svelte';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import Status from '../ui/Status.svelte';
import { transactionToast } from '../ui/transaction-toast.svelte';
import { controlPhase, createTunnelControl } from '../vpn-tunnel.svelte';

const loader = createLoader(fetchIPv6);
const control = createTunnelControl(() => loader.retry(), {
  submit: (enabled, signal) => setIpv6Settings({ body: { enabled }, signal }),
  refusal: ipv6Refusal,
  confirmed: m.ipv6_confirmed,
  rolledBack: m.ipv6_rolled_back,
  rollbackFailed: m.ipv6_rollback_failed,
  failed: m.ipv6_failed,
});
transactionToast(
  'ipv6',
  () => controlPhase(control, m.ipv6_applying()),
  (ok) => void control.settle(ok),
);
const confirm = createConfirm();
onDestroy(() => control.stop());

async function toggle(enabled: boolean) {
  if (control.busy) return;
  const ok = await confirm.ask({
    title: enabled ? m.ipv6_confirm_on_title() : m.ipv6_confirm_off_title(),
    description: m.ipv6_confirm(),
    action: enabled ? m.ipv6_turn_on() : m.ipv6_turn_off(),
  });
  if (ok) void control.set(enabled);
}
</script>

<Card title={m.ipv6_title()}>
  <p class="text-muted-foreground">{m.ipv6_intro()}</p>
  <LoadState status={loader.status} retry={loader.retry} stale={loader.data !== undefined}>
    {#if loader.data}
      {@const on = loader.data.enabled}
      <div class="mt-3 flex flex-wrap items-center gap-3">
        <Status kind={on ? 'ok' : 'neutral'}>{on ? m.ipv6_state_on() : m.ipv6_state_off()}</Status>
        <Button variant="outline" type="button" disabled={control.busy} onclick={() => toggle(!on)}>
          {control.applying ? m.ipv6_applying() : on ? m.ipv6_turn_off() : m.ipv6_turn_on()}
        </Button>
      </div>
    {/if}
  </LoadState>
</Card>

<ConfirmDialog {confirm} />
