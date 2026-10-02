import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';

const streamRequest = vi.hoisted(() => vi.fn());

vi.mock('@/lib/api-client', () => ({
  streamRequest,
}));

import { useInstallLogStream } from '../use-install-log-stream';

type StreamEventLike = { type: string; data?: string };
type Call = {
  route: string;
  onEvent: (e: StreamEventLike) => void;
  signal?: AbortSignal;
};

function lastCall(): Call {
  const [route, onEvent, signal] = streamRequest.mock.calls[
    streamRequest.mock.calls.length - 1
  ] as unknown as [string, Call['onEvent'], AbortSignal];
  return { route, onEvent, signal };
}

function deferred() {
  let resolve!: () => void;
  let reject!: (err: Error) => void;
  const promise = new Promise<void>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function opts(overrides: Partial<Parameters<typeof useInstallLogStream>[0]> = {}) {
  return {
    open: true,
    serviceId: 'tailscale',
    action: 'install' as const,
    onComplete: vi.fn(),
    onOpenChange: vi.fn(),
    ...overrides,
  };
}

beforeEach(() => {
  streamRequest.mockReset();
  streamRequest.mockReturnValue(deferred().promise);
});

describe('useInstallLogStream', () => {
  it('aborts the in-flight request when the dialog closes', async () => {
    const { rerender } = renderHook(
      ({ open }: { open: boolean }) => useInstallLogStream(opts({ open })),
      { initialProps: { open: true } },
    );

    await waitFor(() => expect(streamRequest).toHaveBeenCalledTimes(1));
    const call = lastCall();
    expect(call.route).toBe('/api/v1/services/tailscale/install/stream');
    expect(call.signal?.aborted).toBe(false);

    rerender({ open: false });
    expect(call.signal?.aborted).toBe(true);
  });

  it('aborts on unmount', async () => {
    const { unmount } = renderHook(() => useInstallLogStream(opts()));
    await waitFor(() => expect(streamRequest).toHaveBeenCalledTimes(1));
    const call = lastCall();

    unmount();
    expect(call.signal?.aborted).toBe(true);
  });

  it('drops the previous stream when the dialog is reopened', async () => {
    const { rerender } = renderHook(
      ({ open }: { open: boolean }) => useInstallLogStream(opts({ open })),
      { initialProps: { open: true } },
    );

    await waitFor(() => expect(streamRequest).toHaveBeenCalledTimes(1));
    const first = lastCall();

    rerender({ open: false });
    rerender({ open: true });

    await waitFor(() => expect(streamRequest).toHaveBeenCalledTimes(2));
    const second = lastCall();
    expect(first.signal?.aborted).toBe(true);
    expect(second.signal?.aborted).toBe(false);
    expect(second).not.toBe(first);
  });

  it('shows an error line for a failed stream', async () => {
    const d = deferred();
    streamRequest.mockReturnValue(d.promise);
    const { result } = renderHook(() => useInstallLogStream(opts()));

    await waitFor(() => expect(streamRequest).toHaveBeenCalledTimes(1));
    await act(async () => {
      d.reject(new Error('apk add failed'));
      await d.promise.catch(() => {});
    });

    await waitFor(() => expect(result.current.lines).toEqual(['ERROR: apk add failed']));
    expect(result.current.status).toBe('error');
  });

  it('stays silent when the stream fails because it was aborted', async () => {
    const d = deferred();
    streamRequest.mockReturnValue(d.promise);
    const { result, rerender } = renderHook(
      ({ open }: { open: boolean }) => useInstallLogStream(opts({ open })),
      { initialProps: { open: true } },
    );

    await waitFor(() => expect(streamRequest).toHaveBeenCalledTimes(1));
    const call = lastCall();

    rerender({ open: false });
    await act(async () => {
      d.reject(new Error('The operation was aborted.'));
      await d.promise.catch(() => {});
    });

    expect(call.signal?.aborted).toBe(true);
    expect(result.current.lines).toEqual([]);
    expect(result.current.status).toBe('streaming');
  });
});
