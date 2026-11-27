import type { TailscaleSettings } from './api/gen';
import { changeTailscale, type TailscaleAction } from './api/tailscale';
import { m } from './paraglide/messages.js';
import { notify } from './ui/notify';

/**
 * One Tailscale change at a time, reported in the 'tailscale' toast. The page creates it, not a
 * switch: a reload rebuilds the switch, and that must not abort the request or close the toast.
 */
export function createTailscaleControl(refresh: () => unknown) {
  const request = new AbortController();
  let pending = $state<TailscaleAction>();

  async function change(action: TailscaleAction, settings?: TailscaleSettings) {
    if (pending || request.signal.aborted) return;
    pending = action;
    notify.progress(
      action === 'install' ? m.tailscale_installing() : m.tailscale_applying(),
      'tailscale',
    );
    const result = await changeTailscale(action, request.signal, settings);
    if (request.signal.aborted) return;
    pending = undefined;
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
    change,
    stop,
  };
}

export type TailscaleControl = ReturnType<typeof createTailscaleControl>;
