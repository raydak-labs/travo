import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ReactNode } from 'react';

vi.mock('@/lib/api-client', () => ({
  apiClient: { get: vi.fn(), post: vi.fn(), put: vi.fn(), del: vi.fn() },
}));
vi.mock('@/lib/ws-context', () => ({
  useWsSubscribe: vi.fn(),
}));

import { apiClient } from '@/lib/api-client';
import { useWsSubscribe } from '@/lib/ws-context';
import { useNetworkStatus } from '../use-network';

const mockedGet = vi.mocked(apiClient.get);
const mockedUseWsSubscribe = vi.mocked(useWsSubscribe);

const httpStatus = { internet_reachable: false, wan: null, clients: [] };

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

describe('useNetworkStatus', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockedGet.mockResolvedValue(httpStatus);
  });

  // The WS wiring must live in useNetworkStatus itself, not only in the
  // dashboard's topology hook — every consumer should get live pushes.
  it('feeds WebSocket network_status pushes into the query cache', async () => {
    let capturedHandler: ((data: unknown) => void) | null = null;
    mockedUseWsSubscribe.mockReturnValue({
      connected: true,
      subscribe: (type: string, handler: (data: unknown) => void) => {
        if (type === 'network_status') capturedHandler = handler;
        return () => {};
      },
    });

    const { result } = renderHook(() => useNetworkStatus(), { wrapper });
    await waitFor(() => expect(result.current.data).toBeDefined());

    expect(capturedHandler).not.toBeNull();
    act(() => {
      capturedHandler!({ ...httpStatus, internet_reachable: true });
    });

    await waitFor(() => expect(result.current.data?.internet_reachable).toBe(true));
    expect(mockedGet).toHaveBeenCalledTimes(1);
  });
});

describe('alertsRefetchInterval', () => {
  it('disables polling while the WebSocket delivers alerts', async () => {
    const { alertsRefetchInterval } = await import('../use-alerts');
    expect(alertsRefetchInterval(true)).toBe(false);
    expect(alertsRefetchInterval(false)).toBe(30_000);
  });
});
