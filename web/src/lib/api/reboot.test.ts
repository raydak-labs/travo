import { describe, expect, it, vi } from 'vitest';
import { rebootErrorKey, waitForBoot } from './reboot';

function clock() {
  let elapsed = 0;
  return {
    now: () => elapsed,
    sleep: async (ms: number) => {
      elapsed += ms;
    },
  };
}

describe('waitForBoot', () => {
  it('ignores rejections and the old id, then accepts a different boot id', async () => {
    const health = vi
      .fn<() => Promise<{ boot_id?: string }>>()
      .mockRejectedValueOnce(new Error('down'))
      .mockResolvedValueOnce({ boot_id: 'before' })
      .mockResolvedValueOnce({ boot_id: 'after' });
    const deps = { ...clock(), health };

    await expect(waitForBoot('before', deps, { intervalMs: 2000, timeoutMs: 10000 })).resolves.toBe(
      'restarted',
    );
    expect(health).toHaveBeenCalledTimes(3);
  });

  it('times out when a healthy daemon never changes its boot id', async () => {
    const health = vi.fn(async () => ({ boot_id: 'before' }));
    const deps = { ...clock(), health };

    await expect(waitForBoot('before', deps, { intervalMs: 2000, timeoutMs: 6000 })).resolves.toBe(
      'timeout',
    );
    expect(health).toHaveBeenCalledTimes(3);
  });

  it('ignores empty and missing boot ids', async () => {
    const health = vi
      .fn<() => Promise<{ boot_id?: string }>>()
      .mockResolvedValueOnce({ boot_id: '' })
      .mockResolvedValueOnce({})
      .mockResolvedValueOnce({ boot_id: 'after' });
    const deps = { ...clock(), health };

    await expect(waitForBoot('before', deps, { intervalMs: 2000, timeoutMs: 10000 })).resolves.toBe(
      'restarted',
    );
    expect(health).toHaveBeenCalledTimes(3);
  });

  it('stops quietly on abort without another health call', async () => {
    const controller = new AbortController();
    const health = vi.fn(async () => ({ boot_id: 'before' }));
    let elapsed = 0;
    const result = await waitForBoot(
      'before',
      {
        health,
        now: () => elapsed,
        sleep: async (ms: number) => {
          elapsed += ms;
          controller.abort();
        },
        signal: controller.signal,
      },
      { intervalMs: 2000, timeoutMs: 10000 },
    );

    expect(result).toBe('timeout');
    expect(health).toHaveBeenCalledTimes(1);
  });

  it('passes the abort signal to an in-flight health request', async () => {
    const controller = new AbortController();
    const health = vi.fn(
      (signal?: AbortSignal) =>
        new Promise<{ boot_id?: string }>((_, reject) => {
          if (!signal) {
            reject(new Error('missing abort signal'));
            return;
          }
          signal.addEventListener('abort', () => reject(signal.reason), { once: true });
        }),
    );
    const waiting = waitForBoot('before', {
      ...clock(),
      health,
      signal: controller.signal,
    });

    controller.abort();

    await expect(waiting).resolves.toBe('timeout');
    expect(health).toHaveBeenCalledTimes(1);
    expect(health.mock.calls[0]?.[0]?.aborted).toBe(true);
  });

  it('times out even when a health request ignores abort and never settles', async () => {
    vi.useFakeTimers();
    try {
      const health = vi.fn((_signal: AbortSignal) => new Promise<never>(() => {}));
      const waiting = waitForBoot(
        'before',
        { health, now: Date.now, sleep: async () => {} },
        { intervalMs: 100, timeoutMs: 500 },
      );
      await vi.advanceTimersByTimeAsync(500);
      await expect(waiting).resolves.toBe('timeout');
      expect(health).toHaveBeenCalledTimes(1);
      expect(health.mock.calls[0]?.[0].aborted).toBe(true);
    } finally {
      vi.useRealTimers();
    }
  });

  it('does not call health when the signal is already aborted', async () => {
    const controller = new AbortController();
    controller.abort();
    const health = vi.fn(async () => ({ boot_id: 'after' }));
    await expect(
      waitForBoot('before', { ...clock(), health, signal: controller.signal }),
    ).resolves.toBe('timeout');
    expect(health).not.toHaveBeenCalled();
  });
});

describe('rebootErrorKey', () => {
  it.each([
    ['transaction_pending', 'txn'],
    ['update_busy', 'update'],
    ['reboot_pending', 'pending'],
    ['reboot_failed', 'generic'],
    [undefined, 'generic'],
  ] as const)('maps %s to %s', (code, expected) => {
    expect(rebootErrorKey(code)).toBe(expected);
  });
});
