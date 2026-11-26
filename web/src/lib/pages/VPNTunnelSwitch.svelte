<script lang="ts">
import Switch from '$lib/components/ui/switch/switch.svelte';
import type { TunnelStatus } from '../api/gen';
import { m } from '../paraglide/messages.js';
import ConfirmDialog from '../ui/ConfirmDialog.svelte';
import { createConfirm } from '../ui/confirm.svelte';
import { transactionToast } from '../ui/transaction-toast.svelte';
import { controlPhase, type TunnelControl, tunnelEnabled } from '../vpn-tunnel.svelte';

// The WireGuard tunnel as a switch: on applies through the guarded transaction, off asks first
// because LAN traffic then bypasses the VPN (or is cut by the kill switch).
let {
  id,
  snapshot,
  control,
  canEnable,
}: { id: string; snapshot: TunnelStatus; control: TunnelControl; canEnable: boolean } = $props();

transactionToast(
  'vpn-tunnel',
  () => controlPhase(control, m.vpn_tunnel_applying()),
  (ok) => void control.settle(ok),
);
const confirm = createConfirm();
const enabled = $derived(tunnelEnabled(snapshot.state));

async function change(on: boolean) {
  if (!on) {
    const ok = await confirm.ask({
      title: m.vpn_tunnel_disable_title(),
      description: m.vpn_tunnel_disable_body(),
      action: m.vpn_tunnel_disable(),
      tone: 'danger',
    });
    if (!ok) return;
  }
  void control.set(on);
}
</script>

<Switch
  {id}
  aria-label={m.vpn_tunnel()}
  bind:checked={() => enabled, (on) => void change(on)}
  disabled={control.busy || snapshot.stale || (!enabled && !canEnable)}
/>
<ConfirmDialog {confirm} />
