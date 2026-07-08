import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';

vi.mock('@/lib/api-client', () => ({
  apiClient: { get: vi.fn() },
}));
vi.mock('@/lib/ws-context', () => ({
  useWsSubscribe: vi.fn(),
}));

import { apiClient } from '@/lib/api-client';
import { useWsSubscribe } from '@/lib/ws-context';
import { useSystemStats, statsRefetchInterval } from '../use-system';

const mockedGet = vi.mocked(apiClient.get);
const mockedUseWsSubscribe = vi.mocked(useWsSubscribe);

const httpStats = {
  cpu: { usage_percent: 10, load_average: [0, 0, 0], cores: 4 },
  memory: { total_bytes: 100, used_bytes: 50, free_bytes: 50, cached_bytes: 0, usage_percent: 50 },
  storage: { total_bytes: 0, used_bytes: 0, free_bytes: 0, usage_percent: 0 },
  network: [],
};

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

describe('useSystemStats', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockedGet.mockResolvedValue(httpStats);
  });

  it('feeds WebSocket system_stats pushes into the query cache', async () => {
    let capturedHandler: ((data: unknown) => void) | null = null;
    mockedUseWsSubscribe.mockReturnValue({
      connected: true,
      subscribe: (type: string, handler: (data: unknown) => void) => {
        if (type === 'system_stats') capturedHandler = handler;
        return () => {};
      },
    });

    const { result } = renderHook(() => useSystemStats(), { wrapper });
    await waitFor(() => expect(result.current.data).toBeDefined());

    const wsStats = { ...httpStats, cpu: { ...httpStats.cpu, usage_percent: 77 } };
    expect(capturedHandler).not.toBeNull();
    act(() => {
      capturedHandler!(wsStats);
    });

    await waitFor(() => expect(result.current.data?.cpu.usage_percent).toBe(77));
    // The WS push must not have triggered an extra HTTP fetch.
    expect(mockedGet).toHaveBeenCalledTimes(1);
  });

  it('still fetches over HTTP when the WebSocket is disconnected', async () => {
    mockedUseWsSubscribe.mockReturnValue({
      connected: false,
      subscribe: () => () => {},
    });

    const { result } = renderHook(() => useSystemStats(), { wrapper });
    await waitFor(() => expect(result.current.data).toBeDefined());
    expect(mockedGet).toHaveBeenCalledWith('/api/v1/system/stats');
  });
});

describe('statsRefetchInterval', () => {
  it('disables polling while the WebSocket delivers stats', () => {
    expect(statsRefetchInterval(true)).toBe(false);
  });

  it('polls every 5s as fallback when the WebSocket is down', () => {
    expect(statsRefetchInterval(false)).toBe(5_000);
  });
});
