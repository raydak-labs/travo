<script lang="ts">
import { onDestroy } from 'svelte';
import Button from '$lib/components/ui/button/button.svelte';
import { setIpv6Settings } from '../api/gen';
import { fetchIPv6, ipv6Refusal } from '../api/ipv6';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import LoadState from '../ui/LoadState.svelte';
import { createLoader } from '../ui/loader.svelte';
import Section from '../ui/Section.svelte';
import SettingsRow from '../ui/SettingsRow.svelte';
import Status from '../ui/Status.svelte';
import { transactionToast } from '../ui/transaction-toast.svelte';
import { controlPhase, createTunnelControl } from '../vpn-tunnel.svelte';

const loader = createLoader(fetchIPv6);
const txn = createTunnelControl(() => loader.retry(), {
  submit: (enabled, signal) => setIpv6Settings({ body: { enabled }, signal }),
  refusal: ipv6Refusal,
  confirmed: m.ipv6_confirmed,
  rolledBack: m.ipv6_rolled_back,
  rollbackFailed: m.ipv6_rollback_failed,
  failed: m.ipv6_failed,
});
transactionToast(
  'ipv6',
  () => controlPhase(txn, m.ipv6_applying()),
  (ok) => void txn.settle(ok),
);
const confirm = createConfirm();
onDestroy(() => txn.stop());

async function toggle(enabled: boolean) {
  if (txn.busy) return;
  const ok = await confirm.ask({
    title: enabled ? m.ipv6_confirm_on_title() : m.ipv6_confirm_off_title(),
    description: m.ipv6_confirm(),
    action: enabled ? m.ipv6_turn_on() : m.ipv6_turn_off(),
  });
  if (ok) void txn.set(enabled);
}
</script>

<Section title={m.ipv6_title()} description={m.ipv6_intro()}>
  <LoadState status={loader.status} retry={loader.retry} stale={loader.data !== undefined}>
    {#if loader.data}
      {@const on = loader.data.enabled}
      <SettingsRow label={m.vpn_status_label()}>
        {#snippet control()}
          <Status kind={on ? 'ok' : 'neutral'}>{on ? m.ipv6_state_on() : m.ipv6_state_off()}</Status>
          <Button variant="outline" size="sm" type="button" disabled={txn.busy} onclick={() => toggle(!on)}>
            {txn.applying ? m.ipv6_applying() : on ? m.ipv6_turn_off() : m.ipv6_turn_on()}
          </Button>
        {/snippet}
      </SettingsRow>
    {/if}
  </LoadState>
</Section>

<ConfirmDialog {confirm} />
