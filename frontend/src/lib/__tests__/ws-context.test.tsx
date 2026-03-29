import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import React from 'react';
import { WsProvider, useWsSubscribe } from '../ws-context';

vi.mock('../api-client', () => ({
  getToken: () => 'test-token',
}));

class MockWebSocket {
  static OPEN = 1;
  readyState = MockWebSocket.OPEN;
  onopen: ((e: Event) => void) | null = null;
  onmessage: ((e: MessageEvent) => void) | null = null;
  onclose: ((e: CloseEvent) => void) | null = null;
  onerror: ((e: Event) => void) | null = null;
  close = vi.fn();
  send = vi.fn();
}

let mockWsInstance: MockWebSocket;

beforeEach(() => {
  class WebSocketCtor {
    constructor(...args: unknown[]) {
      void args;
      mockWsInstance = new MockWebSocket();
      return mockWsInstance as unknown as WebSocket;
    }
  }
  vi.stubGlobal('WebSocket', WebSocketCtor as unknown as typeof WebSocket);
});

function wrapper({ children }: { children: React.ReactNode }) {
  return <WsProvider>{children}</WsProvider>;
}

describe('WsProvider', () => {
  it('dispatches message to subscriber matching type', () => {
    const handler = vi.fn();
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });

    act(() => {
      result.current.subscribe('network_status', handler);
    });

    act(() => {
      mockWsInstance.onmessage?.({
        data: JSON.stringify({ type: 'network_status', data: { wan: null } }),
      } as MessageEvent);
    });

    expect(handler).toHaveBeenCalledWith({ wan: null });
  });

  it('does not dispatch to unrelated subscribers', () => {
    const handler = vi.fn();
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });

    act(() => {
      result.current.subscribe('system_stats', handler);
    });

    act(() => {
      mockWsInstance.onmessage?.({
        data: JSON.stringify({ type: 'network_status', data: {} }),
      } as MessageEvent);
    });

    expect(handler).not.toHaveBeenCalled();
  });

  it('unsubscribes correctly', () => {
    const handler = vi.fn();
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });

    let unsub!: () => void;
    act(() => {
      unsub = result.current.subscribe('network_status', handler);
    });
    act(() => {
      unsub();
    });

    act(() => {
      mockWsInstance.onmessage?.({
        data: JSON.stringify({ type: 'network_status', data: {} }),
      } as MessageEvent);
    });

    expect(handler).not.toHaveBeenCalled();
  });

  it('sets connected=true after onopen fires', () => {
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });
    expect(result.current.connected).toBe(false);

    act(() => {
      mockWsInstance.onopen?.(new Event('open'));
    });

    expect(result.current.connected).toBe(true);
  });
});
