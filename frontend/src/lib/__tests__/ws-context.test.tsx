import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import React from 'react';
import { http, HttpResponse } from 'msw/http';
import { API_ROUTES } from '@shared/index';
import { WsProvider, useWsSubscribe } from '../ws-context';
import { server } from '@/mocks/server';
import { clearToken, getToken, setToken, TOKEN_CHANGE_EVENT } from '../api-client';

class MockWebSocket {
  static OPEN = 1;
  url: string;
  protocols?: string | string[];
  readyState = MockWebSocket.OPEN;

  constructor(url = '', protocols?: string | string[]) {
    this.url = url;
    this.protocols = protocols;
  }
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
    constructor(url: string, protocols?: string | string[]) {
      current = new MockWebSocket(url, protocols);
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

async function flushTicket() {
  await act(async () => {
    await Promise.resolve();
  });
}

describe('WsProvider — transport security', () => {
  it('uses a single-use ticket subprotocol and never puts the session JWT in the URL', async () => {
    renderHook(() => useWsSubscribe(), { wrapper });
    await flushTicket();

    expect(sockets).toHaveLength(1);
    expect(sockets[0]!.url).not.toContain('test-token');
    expect(sockets[0]!.url).not.toContain('token=');
    expect(sockets[0]!.protocols).toEqual(['travo-ws-ticket.test-ws-ticket']);
  });

  it('does not open a socket when logout happens during the ticket request', async () => {
    let ticketRequests = 0;
    let ticketSignal: AbortSignal | undefined;
    let releaseTicket!: () => void;
    const pending = new Promise<void>((resolve) => {
      releaseTicket = resolve;
    });
    server.use(
      http.post(API_ROUTES.ws.ticket, async ({ request }) => {
        ticketRequests += 1;
        ticketSignal = request.signal;
        await pending;
        return HttpResponse.json({ ticket: 'late-ticket', expires_in: 30 });
      }),
    );
    renderHook(() => useWsSubscribe(), { wrapper });
    await waitFor(() => expect(ticketRequests).toBe(1));

    act(() => {
      clearToken();
    });
    releaseTicket();
    await act(async () => {
      await Promise.resolve();
    });

    expect(ticketSignal?.aborted).toBe(true);
    expect(sockets).toHaveLength(0);
  });

  it('aborts a ticket request that exceeds 5 seconds', async () => {
    let ticketSignal: AbortSignal | undefined;
    const pending = new Promise<void>(() => {});
    server.use(
      http.post(API_ROUTES.ws.ticket, async ({ request }) => {
        ticketSignal = request.signal;
        await pending;
        return HttpResponse.json({ ticket: 'too-late', expires_in: 30 });
      }),
    );
    renderHook(() => useWsSubscribe(), { wrapper });
    await waitFor(() => expect(ticketSignal).toBeDefined());
    await waitFor(() => expect(ticketSignal?.aborted).toBe(true), { timeout: 6000 });
    expect(sockets).toHaveLength(0);
  }, 7000);
});

describe('WsProvider', () => {
  it('dispatches message to subscriber matching type', async () => {
    const handler = vi.fn();
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });
    await flushTicket();

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

  it('does not dispatch to unrelated subscribers', async () => {
    const handler = vi.fn();
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });
    await flushTicket();

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

  it('unsubscribes correctly', async () => {
    const handler = vi.fn();
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });
    await flushTicket();

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

  it('sets connected=true after onopen fires', async () => {
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });
    await flushTicket();
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

  it('connects after a client-side login and disconnects again on logout', async () => {
    clearToken();
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });
    expect(sockets).toHaveLength(0);

    // login page stores the token without a page reload
    await act(async () => {
      setToken('fresh-token');
    });
    await flushTicket();

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

  it('keeps exactly one socket and ignores a stale socket onclose', async () => {
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });
    await flushTicket();
    vi.useFakeTimers();

    const first = sockets[0];
    act(() => {
      first.onopen?.(new Event('open'));
    });
    expect(result.current.connected).toBe(true);

    // Simulate a StrictMode-style remount: cleanup closes the old socket
    // without detaching handlers reaching state, then a new socket opens.
    const staleClose = first.onclose;
    const staleOpen = first.onopen;
    vi.useRealTimers();
    await act(async () => {
      setToken('rotated-token');
    });
    await flushTicket();
    vi.useFakeTimers();
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

  it('reconnects after an unexpected close', async () => {
    let ticketRequests = 0;
    server.use(
      http.post(API_ROUTES.ws.ticket, () => {
        ticketRequests += 1;
        return HttpResponse.json({ ticket: `ticket-${ticketRequests}`, expires_in: 30 });
      }),
    );
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });
    await flushTicket();
    expect(sockets).toHaveLength(1);

    act(() => {
      sockets[0].onopen?.(new Event('open'));
    });
    act(() => {
      sockets[0].onclose?.(new CloseEvent('close'));
    });
    expect(result.current.connected).toBe(false);
    expect(getToken()).toBe('test-token');

    await waitFor(() => expect(ticketRequests).toBe(2), { timeout: 4000 });
    await waitFor(() => expect(sockets).toHaveLength(2));
  });

  it('does not reconnect when the session was revoked while offline', async () => {
    const { result } = renderHook(() => useWsSubscribe(), { wrapper });
    await flushTicket();
    vi.useFakeTimers();

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
