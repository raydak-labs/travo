<script lang="ts">
import { onDestroy } from 'svelte';
import Switch from '$lib/components/ui/switch/switch.svelte';
import type { TailscaleStatus } from '../api/gen';
import { changeTailscale } from '../api/tailscale';
import { m } from '../paraglide/messages.js';
import { notify } from '../ui/notify';

// Tailscale on/off as a row switch; the Tailscale page keeps install, login and options.
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

const request = new AbortController();
let pending = $state(false);
onDestroy(() => {
  request.abort();
  notify.dismiss('tailscale');
});

async function toggle(on: boolean) {
  if (pending) return;
  pending = true;
  notify.progress(m.tailscale_applying(), 'tailscale');
  const result = await changeTailscale(on ? 'on' : 'off', request.signal);
  if (request.signal.aborted) return;
  pending = false;
  if ('error' in result) notify.error(result.error, 'tailscale');
  else notify.success(m.tailscale_done(), 'tailscale');
  await refresh();
}
</script>

<Switch {id} aria-label={m.tailscale_running_label()} disabled={pending || disabled || status.busy}
  bind:checked={() => status.running, (on) => void toggle(on)} />
