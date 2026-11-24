import { postSystemReboot } from './api/gen';
import './api/client';
import { fetchHealth } from './api/health';
import { rebootErrorKey, waitForBoot } from './api/reboot';
import { m } from './paraglide/messages.js';
import type { createConfirm } from './ui/confirm.svelte';
import { notify } from './ui/notify';

// One restart for the whole app: System → Device and the header menu share it, and it keeps
// waiting for the router to come back when the user leaves the page that started it.
// Progress and outcome are toasts (id 'reboot'); callers only block a second request.
export type RestartState = 'idle' | 'requesting' | 'restarting' | 'done';

let state = $state<RestartState>('idle');

function failed(key: ReturnType<typeof rebootErrorKey>) {
  state = 'idle';
  notify.error(m.reboot_error({ key }), 'reboot');
}

async function run() {
  if (state !== 'idle') return;
  state = 'requesting';
  try {
    const { data, error } = await postSystemReboot({ body: { confirm: true } });
    if (!data) {
      failed(rebootErrorKey((error as { code?: string } | undefined)?.code));
      return;
    }
    state = 'restarting';
    notify.progress(m.reboot_restarting(), 'reboot');
    const result = await waitForBoot(data.boot_id, {
      health: fetchHealth,
      sleep: (ms) => new Promise<void>((resolve) => setTimeout(resolve, ms)),
      now: Date.now,
      signal: new AbortController().signal,
    });
    state = 'done';
    if (result === 'restarted') notify.success(m.reboot_back(), 'reboot');
    else notify.error(m.reboot_timeout(), 'reboot');
  } catch {
    failed('generic');
  }
}

export const restart = {
  get state(): RestartState {
    return state;
  },
  /** Asks through the caller's ConfirmDialog, then restarts. */
  async ask(confirm: ReturnType<typeof createConfirm>) {
    const ok = await confirm.ask({
      title: m.reboot_confirm_title(),
      description: m.reboot_confirm(),
      action: m.reboot_router(),
      tone: 'danger',
    });
    if (ok) await run();
  },
};
