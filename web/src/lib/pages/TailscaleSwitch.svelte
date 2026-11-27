<script lang="ts">
import { onDestroy } from 'svelte';
import Switch from '$lib/components/ui/switch/switch.svelte';
import type { TailscaleStatus } from '../api/gen';
import { m } from '../paraglide/messages.js';
import { createTailscaleToggle } from '../tailscale-toggle.svelte';

let {
  id,
  status,
  disabled = false,
  refresh,
}: {
  id: string;
  status: TailscaleStatus;
  disabled?: boolean;
  refresh: () => void | Promise<void>;
} = $props();

const control = createTailscaleToggle(() => refresh());
onDestroy(control.stop);
</script>

<Switch {id} aria-label={m.tailscale_running_label()} disabled={control.pending || disabled || status.busy}
  bind:checked={() => status.running, (on) => void control.toggle(on)} />
