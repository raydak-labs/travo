import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import {
  QueryClient,
  QueryClientProvider,
  focusManager,
  onlineManager,
} from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import type { ReactNode } from 'react';
import { server } from '@/mocks/server';
import { API_ROUTES } from '@shared/index';
import type { NetworkInterface, NetworkInterfaceType, NetworkStatus } from '@shared/index';
import { mockNetworkStatus } from '@/mocks/data';
import { setToken } from '@/lib/api-client';

vi.mock('@/lib/ws-context', () => ({
  useWsSubscribe: vi.fn(),
}));

import { useWsSubscribe } from '@/lib/ws-context';
import { useTopologyData, topologyRefetchInterval } from '../use-topology-data';

const mockedUseWsSubscribe = vi.mocked(useWsSubscribe);

let queryClient: QueryClient;

function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

function setWs({ connected }: { connected: boolean }) {
  mockedUseWsSubscribe.mockReturnValue({
    connected,
    subscribe: () => () => {},
  });
}

function makeIface(type: NetworkInterfaceType, up: boolean, name = 'wan'): NetworkInterface {
  return {
    name,
    type,
    ip_address: '1.2.3.4',
    netmask: '255.255.255.0',
    gateway: '1.2.3.1',
    dns_servers: [],
    mac_address: '',
    is_up: up,
    rx_bytes: 0,
    tx_bytes: 0,
  };
}

function statusWithWan(wan: NetworkInterface | null): NetworkStatus {
  return { ...mockNetworkStatus, wan };
}

beforeEach(() => {
  vi.clearAllMocks();
  setToken('test-token');
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
});

afterEach(() => {
  focusManager.setFocused(undefined);
  onlineManager.setOnline(true);
  vi.useRealTimers();
});

describe('useTopologyData — connection type derivation', () => {
  // `type` is the medium discriminator reported by the router, not a protocol
  // and not a UCI section name. A USB-tethered uplink must not be reported as
  // Ethernet, and a VPN/LAN entry is not an upstream either.
  const cases = [
    { type: 'wan' as const, up: true, ethernet: true, repeater: false, tether: false },
    { type: 'wan' as const, up: false, ethernet: false, repeater: false, tether: false },
    { type: 'wifi' as const, up: true, ethernet: false, repeater: true, tether: false },
    { type: 'usb' as const, up: true, ethernet: false, repeater: false, tether: true },
    { type: 'lan' as const, up: true, ethernet: false, repeater: false, tether: false },
    { type: 'vpn' as const, up: true, ethernet: false, repeater: false, tether: false },
  ];

  cases.forEach(({ type, up, ethernet, repeater, tether }) => {
    it(`wan.type=${type} wan.is_up=${up} → eth=${ethernet} rep=${repeater} tether=${tether}`, async () => {
      setWs({ connected: true });
      server.use(
        http.get(API_ROUTES.network.status, () =>
          HttpResponse.json(statusWithWan(makeIface(type, up))),
        ),
      );
      // No WiFi STA / USB tethering service data, so only the WAN decides.
      server.use(
        http.get(API_ROUTES.wifi.connection, () =>
          HttpResponse.json({ ...mockNetworkStatus.wan, connected: false, mode: 'ap' }),
        ),
      );

      const { result } = renderHook(() => useTopologyData(), { wrapper });

      await waitFor(() => expect(result.current.wan).not.toBeNull());
      expect(result.current.ethernetUp).toBe(ethernet);
      expect(result.current.repeaterUp).toBe(repeater);
      expect(result.current.tetherUp).toBe(tether);
    });
  });

  it('reports the WAN protocol from the WAN config, not from the interface type', async () => {
    setWs({ connected: true });
    server.use(
      http.get(API_ROUTES.network.status, () =>
        HttpResponse.json(statusWithWan(makeIface('wan', true))),
      ),
    );

    const { result } = renderHook(() => useTopologyData(), { wrapper });

    await waitFor(() => expect(result.current.ethernetUp).toBe(true));
    await waitFor(() => expect(result.current.wanProtocol).toBe('dhcp'));
    expect(result.current.wanMedium).toBe('ethernet');
  });
});

describe('useTopologyData — live updates and healing', () => {
  it('feeds WebSocket network_status pushes into the cache without an extra fetch', async () => {
    let captured: ((data: unknown) => void) | null = null;
    mockedUseWsSubscribe.mockReturnValue({
      connected: true,
      subscribe: (type: string, handler: (data: unknown) => void) => {
        if (type === 'network_status') captured = handler;
        return () => {};
      },
    });

    let statusFetches = 0;
    server.use(
      http.get(API_ROUTES.network.status, () => {
        statusFetches += 1;
        return HttpResponse.json(statusWithWan(makeIface('wan', false)));
      }),
    );

    const { result } = renderHook(() => useTopologyData(), { wrapper });
    await waitFor(() => expect(statusFetches).toBe(1));
    expect(result.current.ethernetUp).toBe(false);

    expect(captured).not.toBeNull();
    act(() => {
      captured!(statusWithWan(makeIface('usb', true, 'usb0')));
    });

    await waitFor(() => expect(result.current.ethernetUp).toBe(false));
    expect(result.current.tetherUp).toBe(true);
    expect(result.current.wanMedium).toBe('usb');
    expect(statusFetches).toBe(1);
  });

  it('refetches over HTTP when the WebSocket is disconnected', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    setWs({ connected: false });

    let statusFetches = 0;
    server.use(
      http.get(API_ROUTES.network.status, () => {
        statusFetches += 1;
        return HttpResponse.json(statusWithWan(makeIface('wan', true)));
      }),
    );

    const { result } = renderHook(() => useTopologyData(), { wrapper });

    await waitFor(() => expect(statusFetches).toBe(1));
    await waitFor(() => expect(result.current.ethernetUp).toBe(true));

    // Polling keeps the dashboard healing while the socket is down.
    await vi.advanceTimersByTimeAsync(15_000);
    expect(statusFetches).toBeGreaterThan(1);
  });

  it('refetches when the window regains focus or the network reconnects', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    setWs({ connected: true });

    let statusFetches = 0;
    server.use(
      http.get(API_ROUTES.network.status, () => {
        statusFetches += 1;
        return HttpResponse.json(statusWithWan(makeIface('wan', true)));
      }),
    );

    renderHook(() => useTopologyData(), { wrapper });
    await vi.waitFor(() => expect(statusFetches).toBe(1));

    // staleTime is finite, so a refetch is not suppressed once the data ages.
    await vi.advanceTimersByTimeAsync(6_000);
    expect(statusFetches).toBe(1);

    await act(async () => {
      focusManager.setFocused(false);
      focusManager.setFocused(true);
    });
    await vi.waitFor(() => expect(statusFetches).toBe(2));

    await vi.advanceTimersByTimeAsync(6_000);
    expect(statusFetches).toBe(2);

    await act(async () => {
      onlineManager.setOnline(false);
      onlineManager.setOnline(true);
    });
    await vi.waitFor(() => expect(statusFetches).toBe(3));
  });
});

describe('topologyRefetchInterval', () => {
  it('stops polling while the WebSocket pushes network_status', () => {
    expect(topologyRefetchInterval(true)).toBe(false);
  });

  it('polls as a fallback while the WebSocket is down', () => {
    expect(topologyRefetchInterval(false)).toBe(15_000);
  });
});
