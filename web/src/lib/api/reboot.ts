type ReadBootHealth = (signal: AbortSignal) => Promise<{ boot_id?: string }>;

async function readHealth(health: ReadBootHealth, timeoutMs: number, signal?: AbortSignal) {
  const controller = new AbortController();
  const abort = () => controller.abort();
  signal?.addEventListener('abort', abort, { once: true });
  const timeout = setTimeout(abort, timeoutMs);
  const interrupted = new Promise<undefined>((resolve) => {
    controller.signal.addEventListener('abort', () => resolve(undefined), { once: true });
  });
  try {
    return await Promise.race([interrupted, health(controller.signal)]);
  } finally {
    clearTimeout(timeout);
    signal?.removeEventListener('abort', abort);
  }
}

export async function waitForBoot(
  before: string,
  deps: {
    health: ReadBootHealth;
    sleep: (ms: number) => Promise<void>;
    now: () => number;
    signal?: AbortSignal;
  },
  opts = { intervalMs: 2000, timeoutMs: 180000 },
): Promise<'restarted' | 'timeout'> {
  const started = deps.now();
  while (!deps.signal?.aborted && deps.now() - started < opts.timeoutMs) {
    try {
      const remaining = opts.timeoutMs - (deps.now() - started);
      const health = await readHealth(deps.health, Math.min(4000, remaining), deps.signal);
      if (
        !deps.signal?.aborted &&
        deps.now() - started < opts.timeoutMs &&
        typeof health?.boot_id === 'string' &&
        health.boot_id !== '' &&
        health.boot_id !== before
      ) {
        return 'restarted';
      }
    } catch {
      // The router is expected to reject requests while it is down.
    }
    if (deps.signal?.aborted) return 'timeout';
    const remaining = opts.timeoutMs - (deps.now() - started);
    if (remaining <= 0) return 'timeout';
    await deps.sleep(Math.min(opts.intervalMs, remaining));
  }
  return 'timeout';
}

export function rebootErrorKey(code: string | undefined): 'txn' | 'update' | 'pending' | 'generic' {
  switch (code) {
    case 'transaction_pending':
      return 'txn';
    case 'update_busy':
      return 'update';
    case 'reboot_pending':
      return 'pending';
    default:
      return 'generic';
  }
}
