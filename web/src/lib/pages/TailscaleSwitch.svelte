<script lang="ts">
import Switch from '$lib/components/ui/switch/switch.svelte';
import type { TailscaleStatus } from '../api/gen';
import { m } from '../paraglide/messages.js';
import type { TailscaleControl } from '../tailscale-control.svelte';

let {
  id,
  status,
  control,
  disabled = false,
}: {
  id: string;
  status: TailscaleStatus;
  control: TailscaleControl;
  disabled?: boolean;
} = $props();
</script>

<Switch {id} aria-label={m.tailscale_running_label()}
  disabled={control.pending !== undefined || disabled || status.busy}
  bind:checked={() => status.running, (on) => void control.change(on ? 'on' : 'off')} />
