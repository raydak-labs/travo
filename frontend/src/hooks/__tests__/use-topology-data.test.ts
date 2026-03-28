import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook } from '@testing-library/react';
import React from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { WsProvider } from '@/lib/ws-context';
import { useTopologyData } from '../use-topology-data';
import type { NetworkStatus } from '@shared/index';

vi.mock('@/lib/api-client', () => ({
  getToken: () => 'test-token',
  apiClient: {},
}));

beforeEach(() => {
  vi.stubGlobal(
    'WebSocket',
    vi.fn().mockImplementation(() => ({
      readyState: 1,
      close: vi.fn(),
      onopen: null,
      onmessage: null,
      onclose: null,
      onerror: null,
    })),
  );
});

vi.mock('@/hooks/use-network', () => ({
  useNetworkStatus: vi.fn(() => ({ data: undefined, isLoading: false })),
  useIPv6Status: vi.fn(() => ({ data: undefined })),
}));
vi.mock('@/hooks/use-wifi', () => ({
  useWifiConnection: vi.fn(() => ({ data: undefined, isLoading: false })),
}));
vi.mock('@/hooks/use-vpn', () => ({
  useVpnStatus: vi.fn(() => ({ data: undefined })),
}));
vi.mock('@/hooks/use-system', () => ({
  useSystemInfo: vi.fn(() => ({ data: undefined, isLoading: false })),
}));
vi.mock('@/hooks/use-usb-tether', () => ({
  useUSBTetherStatus: vi.fn(() => ({ data: undefined })),
}));

import { useNetworkStatus } from '@/hooks/use-network';

function makeWrapper() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return React.createElement(
      QueryClientProvider,
      { client: queryClient },
      React.createElement(WsProvider, null, children),
    );
  };
}

function makeWan(type: string, up: boolean): NetworkStatus['wan'] {
  return {
    name: 'wan',
    type: type as never,
    ip_address: '1.2.3.4',
    netmask: '',
    gateway: '',
    dns_servers: [],
    mac_address: '',
    is_up: up,
    rx_bytes: 0,
    tx_bytes: 0,
  };
}

describe('useTopologyData — connection type derivation', () => {
  const cases = [
    { wanType: 'wan',  wanUp: true,  ethernet: true,  repeater: false, tether: false },
    { wanType: 'wifi', wanUp: true,  ethernet: false, repeater: true,  tether: false },
    { wanType: 'usb',  wanUp: true,  ethernet: false, repeater: false, tether: true  },
    { wanType: 'wan',  wanUp: false, ethernet: false, repeater: false, tether: false },
  ];

  cases.forEach(({ wanType, wanUp, ethernet, repeater, tether }) => {
    it(`wan.type=${wanType} wan.is_up=${wanUp} → eth=${ethernet} rep=${repeater} tether=${tether}`, () => {
      const ns: Partial<NetworkStatus> = {
        wan: makeWan(wanType, wanUp),
        clients: [],
        internet_reachable: false,
        lan: makeWan('lan', true) as never,
        interfaces: [],
      };
      vi.mocked(useNetworkStatus).mockReturnValue({
        data: ns as NetworkStatus,
        isLoading: false,
      } as never);

      const { result } = renderHook(() => useTopologyData(), { wrapper: makeWrapper() });

      expect(result.current.ethernetUp).toBe(ethernet);
      expect(result.current.repeaterUp).toBe(repeater);
      expect(result.current.tetherUp).toBe(tether);
    });
  });
});
