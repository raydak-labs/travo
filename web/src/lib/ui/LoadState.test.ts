import { createRawSnippet } from 'svelte';
import { render } from 'svelte/server';
import { describe, expect, it, vi } from 'vitest';
import LoadState from './LoadState.svelte';
import { createLoader, type LoadStatus } from './loader.svelte';

const content = createRawSnippet(() => ({ render: () => '<p>CONTENT</p>' }));

function html(status: LoadStatus, stale = false): string {
  return render(LoadState, { props: { status, retry: () => {}, stale, children: content } }).body;
}

describe('LoadState', () => {
  it('renders a skeleton while loading, not content', () => {
    const out = html({ state: 'loading' });
    expect(out).toContain('skeleton');
    expect(out).not.toContain('CONTENT');
  });

  it('renders the error and a retry button on failure, never content or nothing', () => {
    const out = html({ state: 'error', message: 'boom' });
    expect(out).toContain('role="alert"');
    expect(out).toContain('boom');
    expect(out).toContain('<button');
    expect(out).not.toContain('CONTENT');
    // A plain row inside the surrounding card, not a second bordered Alert box.
    expect(out).not.toContain('data-slot="alert"');
  });

  it('keeps the known value, dimmed, beside the error when stale', () => {
    const out = html({ state: 'error', message: 'boom' }, true);
    expect(out).toContain('CONTENT');
    expect(out).toContain('opacity-60');
    expect(out).toContain('boom');
    expect(out).toContain('Last known value');
    expect(out).toContain('<button');
  });

  it('keeps the known value, dimmed, while a retry loads', () => {
    const out = html({ state: 'loading' }, true);
    expect(out).toContain('CONTENT');
    expect(out).toContain('opacity-60');
    expect(out).not.toContain('skeleton');
  });

  it('renders only the error when not stale', () => {
    const out = html({ state: 'error', message: 'boom' }, false);
    expect(out).not.toContain('CONTENT');
    expect(out).not.toContain('Last known value');
  });

  it('renders content when ready', () => {
    expect(html({ state: 'ready' })).toContain('CONTENT');
  });

  it('turns a rejection into an error state and recovers on retry', async () => {
    const fn = vi.fn().mockRejectedValueOnce(new Error('down')).mockResolvedValueOnce('up');
    const loader = createLoader(fn);
    expect(loader.status.state).toBe('loading');
    await vi.waitFor(() => expect(loader.status).toEqual({ state: 'error', message: 'down' }));
    expect(html(loader.status)).toContain('down');
    await loader.retry();
    expect(loader.status.state).toBe('ready');
    expect(loader.data).toBe('up');
  });

  it('ignores a slow earlier run that resolves after a newer retry', async () => {
    let resolveA: (v: string) => void = () => {};
    const fn = vi
      .fn()
      .mockImplementationOnce(() => new Promise<string>((r) => (resolveA = r)))
      .mockResolvedValueOnce('B');
    const loader = createLoader(fn);
    await loader.retry();
    expect(loader.data).toBe('B');
    resolveA('A');
    await Promise.resolve();
    await Promise.resolve();
    expect(loader.data).toBe('B');
    expect(loader.status.state).toBe('ready');
  });

  it('ignores a slow earlier failure after a newer retry succeeded', async () => {
    let rejectA: (e: Error) => void = () => {};
    const fn = vi
      .fn()
      .mockImplementationOnce(() => new Promise<string>((_, r) => (rejectA = r)))
      .mockResolvedValueOnce('B');
    const loader = createLoader(fn);
    await loader.retry();
    rejectA(new Error('late'));
    await Promise.resolve();
    await Promise.resolve();
    expect(loader.status.state).toBe('ready');
    expect(loader.data).toBe('B');
  });

  it('keeps existing data ready while an interval refresh is in flight', async () => {
    vi.useFakeTimers();
    try {
      let resolvePoll: (value: string) => void = () => {};
      const fn = vi
        .fn()
        .mockResolvedValueOnce('initial')
        .mockImplementationOnce(() => new Promise<string>((resolve) => (resolvePoll = resolve)));
      const loader = createLoader(fn, 10000);
      await vi.advanceTimersByTimeAsync(0);
      expect(loader.status).toEqual({ state: 'ready' });
      expect(loader.data).toBe('initial');

      await vi.advanceTimersByTimeAsync(10000);
      expect(fn).toHaveBeenCalledTimes(2);
      expect(loader.data).toBe('initial');
      expect(loader.status).toEqual({ state: 'ready' });

      resolvePoll('refreshed');
      await vi.advanceTimersByTimeAsync(0);
      expect(loader.data).toBe('refreshed');
      expect(loader.status).toEqual({ state: 'ready' });
      loader.stop();
    } finally {
      vi.useRealTimers();
    }
  });

  it('polls at the requested interval until stopped', async () => {
    vi.useFakeTimers();
    try {
      const fn = vi.fn().mockResolvedValue('up');
      const loader = createLoader(fn, 10000);
      await vi.advanceTimersByTimeAsync(0);
      expect(fn).toHaveBeenCalledTimes(1);
      await vi.advanceTimersByTimeAsync(10000);
      expect(fn).toHaveBeenCalledTimes(2);
      loader.stop();
      await vi.advanceTimersByTimeAsync(20000);
      expect(fn).toHaveBeenCalledTimes(2);
    } finally {
      vi.useRealTimers();
    }
  });

  it('publishes a slow request without overlapping the next scheduled poll', async () => {
    vi.useFakeTimers();
    try {
      let resolveSlow: (value: string) => void = () => {};
      const fn = vi
        .fn()
        .mockImplementationOnce(() => new Promise<string>((resolve) => (resolveSlow = resolve)));
      const loader = createLoader(fn, 10000);

      await vi.advanceTimersByTimeAsync(10000);
      expect(fn).toHaveBeenCalledTimes(1);
      await vi.advanceTimersByTimeAsync(5000);
      resolveSlow('slow but successful');
      await vi.advanceTimersByTimeAsync(0);

      expect(loader.data).toBe('slow but successful');
      expect(loader.status).toEqual({ state: 'ready' });
      loader.stop();
    } finally {
      vi.useRealTimers();
    }
  });

  it('aborts a hung request and recovers on a later scheduled poll', async () => {
    vi.useFakeTimers();
    try {
      const signals: (AbortSignal | undefined)[] = [];
      const fn = vi
        .fn()
        .mockImplementationOnce((signal?: AbortSignal) => {
          signals.push(signal);
          return new Promise<string>(() => {});
        })
        .mockResolvedValueOnce('recovered');
      const loader = createLoader(fn, 10000);

      await vi.advanceTimersByTimeAsync(10000);
      expect(fn).toHaveBeenCalledTimes(1);
      expect(signals[0]?.aborted).toBe(false);
      await vi.advanceTimersByTimeAsync(10000);

      expect(signals[0]?.aborted).toBe(true);
      expect(fn).toHaveBeenCalledTimes(2);
      expect(loader.data).toBe('recovered');
      expect(loader.status).toEqual({ state: 'ready' });
      loader.stop();
    } finally {
      vi.useRealTimers();
    }
  });
});
