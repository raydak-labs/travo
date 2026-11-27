import { changeTailscale } from './api/tailscale';
import { m } from './paraglide/messages.js';
import { notify } from './ui/notify';

/**
 * Tailscale on/off for a row switch. `stop` runs when the switch is destroyed, which a
 * loading refresh does too: only an in-flight request takes its toast with it.
 */
export function createTailscaleToggle(refresh: () => unknown) {
  const request = new AbortController();
  let pending = $state(false);

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

  function stop() {
    request.abort();
    if (pending) notify.dismiss('tailscale');
  }

  return {
    get pending() {
      return pending;
    },
    toggle,
    stop,
  };
}
