import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import React from 'react';
import { WsProvider, useWsSubscribe } from '../ws-context';
import { clearToken, setToken, TOKEN_CHANGE_EVENT } from '../api-client';

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

let sockets: MockWebSocket[] = [];
let current: MockWebSocket;

beforeEach(() => {
  sockets = [];
  class WebSocketCtor {
    constructor(...args: unknown[]) {
      void args;
      current = new MockWebSocket();
      sockets.push(current);
      return current as unknown as WebSocket;
    }
  }
  vi.stubGlobal('WebSocket', WebSocketCtor as unknown as typeof WebSocket);
  setToken('test-token');
});

afterEach(() => {
  clearToken();
  vi.useRealTimers();
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
      current.onmessage?.({
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
      current.onmessage?.({
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
      current.onmessage?.({
        data: JSON.stringify({ type: 'network_status', data: {} }),
      } as MessageEvent);
    });

    expect(handler).not.toHaveBeenCalled();
  });

  it('sets connected=true after onopen fires', () => {
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });
    expect(result.current.connected).toBe(false);

    act(() => {
      current.onopen?.(new Event('open'));
    });

    expect(result.current.connected).toBe(true);
  });
});

describe('WsProvider — auth token lifecycle', () => {
  it('opens no socket while there is no session', () => {
    clearToken();
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });

    expect(sockets).toHaveLength(0);
    expect(result.current.connected).toBe(false);
  });

  it('connects after a client-side login and disconnects again on logout', () => {
    clearToken();
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });
    expect(sockets).toHaveLength(0);

    // login page stores the token without a page reload
    act(() => {
      setToken('fresh-token');
    });

    expect(sockets).toHaveLength(1);
    act(() => {
      sockets[0].onopen?.(new Event('open'));
    });
    expect(result.current.connected).toBe(true);

    act(() => {
      clearToken();
    });

    expect(sockets[0].close).toHaveBeenCalled();
    expect(result.current.connected).toBe(false);
  });

  it('keeps exactly one socket and ignores a stale socket onclose', () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });

    const first = sockets[0];
    act(() => {
      first.onopen?.(new Event('open'));
    });
    expect(result.current.connected).toBe(true);

    // Simulate a StrictMode-style remount: cleanup closes the old socket
    // without detaching handlers reaching state, then a new socket opens.
    const staleClose = first.onclose;
    const staleOpen = first.onopen;
    act(() => {
      setToken('rotated-token');
    });
    expect(sockets).toHaveLength(2);
    act(() => {
      sockets[1].onopen?.(new Event('open'));
    });
    expect(result.current.connected).toBe(true);

    // The obsolete socket closing must not disconnect the live one nor
    // schedule a third socket.
    act(() => {
      staleClose?.(new CloseEvent('close'));
      staleOpen?.(new Event('open'));
    });
    expect(result.current.connected).toBe(true);

    act(() => {
      vi.advanceTimersByTime(10_000);
    });
    expect(sockets).toHaveLength(2);
  });

  it('reconnects after an unexpected close', () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });
    expect(sockets).toHaveLength(1);

    act(() => {
      sockets[0].onopen?.(new Event('open'));
    });
    act(() => {
      sockets[0].onclose?.(new CloseEvent('close'));
    });
    expect(result.current.connected).toBe(false);

    act(() => {
      vi.advanceTimersByTime(3000);
    });
    expect(sockets).toHaveLength(2);
  });

  it('does not reconnect when the session was revoked while offline', () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });

    act(() => {
      sockets[0].onclose?.(new CloseEvent('close'));
      clearToken();
    });

    act(() => {
      vi.advanceTimersByTime(60_000);
    });
    expect(sockets).toHaveLength(1);
    expect(result.current.connected).toBe(false);
  });

  it('uses the token change event as the session signal', () => {
    clearToken();
    renderHook(() => useWsSubscribe(), { wrapper });
    expect(sockets).toHaveLength(0);

    act(() => {
      window.dispatchEvent(new Event(TOKEN_CHANGE_EVENT));
    });
    expect(sockets).toHaveLength(0);
  });
});
