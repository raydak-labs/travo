export type LoadStatus =
  | { state: 'loading' }
  | { state: 'error'; message: string }
  | { state: 'ready' };

const ONE_OFF_TIMEOUT_MS = 5_000;

type ActiveRun = {
  id: number;
  controller: AbortController;
  startedAt: number;
  timeout: ReturnType<typeof setTimeout>;
};

/** Runs `fn`, tracking loading/error/data. A rejection is an error state, never "no data". */
export function createLoader<T>(fn: (signal: AbortSignal) => Promise<T>, interval?: number) {
  let status = $state<LoadStatus>({ state: 'loading' });
  let data = $state<T>();
  // Polls get two intervals: a slow request can publish after one skipped tick, while
  // the next tick can supersede a hung request. One-off loads use a fixed five seconds.
  const requestTimeout = interval === undefined ? ONE_OFF_TIMEOUT_MS : interval * 2;

  // Only the latest run may write state: a slow earlier attempt must not overwrite a retry.
  let latest = 0;
  let active: ActiveRun | undefined;
  const timer =
    interval === undefined
      ? undefined
      : setInterval(() => {
          if (active && Date.now() - active.startedAt < requestTimeout) return;
          void run(false);
        }, interval);

  async function run(announceLoading = true): Promise<void> {
    const mine = ++latest;
    if (active) {
      clearTimeout(active.timeout);
      active.controller.abort();
    }
    const controller = new AbortController();
    const current = {
      id: mine,
      controller,
      startedAt: Date.now(),
      timeout: setTimeout(() => controller.abort(), requestTimeout),
    };
    active = current;
    if (announceLoading) status = { state: 'loading' };
    try {
      const result = await fn(controller.signal);
      if (mine !== latest) return;
      data = result;
      status = { state: 'ready' };
    } catch (e) {
      if (mine !== latest) return;
      status = { state: 'error', message: e instanceof Error ? e.message : String(e) };
    } finally {
      clearTimeout(current.timeout);
      if (active === current) active = undefined;
    }
  }

  void run();

  return {
    get status() {
      return status;
    },
    get data() {
      return data;
    },
    retry: () => run(),
    stop: () => {
      clearInterval(timer);
      latest++;
      if (active) {
        clearTimeout(active.timeout);
        active.controller.abort();
        active = undefined;
      }
    },
  };
}
