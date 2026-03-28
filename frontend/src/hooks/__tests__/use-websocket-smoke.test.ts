import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import React from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { WsProvider } from '@/lib/ws-context';
import { useWebSocket } from '../use-websocket';

vi.mock('@/lib/api-client', () => ({
  getToken: () => 'test-token',
  apiClient: {},
}));

let mockWsInstance: {
  onopen: ((e: Event) => void) | null;
  onmessage: ((e: MessageEvent) => void) | null;
  onclose: ((e: CloseEvent) => void) | null;
  onerror: ((e: Event) => void) | null;
  close: ReturnType<typeof vi.fn>;
};

beforeEach(() => {
  vi.stubGlobal(
    'WebSocket',
    vi.fn().mockImplementation(() => {
      mockWsInstance = {
        onopen: null,
        onmessage: null,
        onclose: null,
        onerror: null,
        close: vi.fn(),
      };
      return mockWsInstance;
    }),
  );
});

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient();
  return React.createElement(
    QueryClientProvider,
    { client: qc },
    React.createElement(WsProvider, null, children),
  );
}

const statsMsg = {
  type: 'system_stats',
  data: {
    cpu: { usage_percent: 42, cores: 4, load_average: [0, 0, 0] },
    memory: {
      total_bytes: 1000,
      used_bytes: 500,
      free_bytes: 500,
      cached_bytes: 0,
      usage_percent: 50,
    },
    storage: { total_bytes: 1000, used_bytes: 200, free_bytes: 800, usage_percent: 20 },
    network: [{ interface: 'eth0', rx_bytes: 100, tx_bytes: 200 }],
  },
};

describe('useWebSocket (after WsContext migration)', () => {
  it('accumulates dataPoints when system_stats messages arrive', () => {
    const { result } = renderHook(() => useWebSocket(), { wrapper });

    act(() => {
      mockWsInstance.onmessage?.({
        data: JSON.stringify(statsMsg),
      } as MessageEvent);
    });

    expect(result.current.dataPoints).toHaveLength(1);
    expect(result.current.dataPoints[0].cpu).toBe(42);
  });

  it('clears dataPoints when WS disconnects', () => {
    const { result } = renderHook(() => useWebSocket(), { wrapper });

    // Establish connection so connected transitions true → false on close
    act(() => {
      mockWsInstance.onopen?.(new Event('open'));
    });

    act(() => {
      mockWsInstance.onmessage?.({ data: JSON.stringify(statsMsg) } as MessageEvent);
    });
    expect(result.current.dataPoints).toHaveLength(1);

    act(() => {
      mockWsInstance.onclose?.({} as CloseEvent);
    });

    expect(result.current.dataPoints).toHaveLength(0);
  });
});
