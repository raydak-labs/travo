import React, { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react';
import { getToken } from './api-client';

type MessageHandler = (data: unknown) => void;

interface WsContextValue {
  connected: boolean;
  subscribe: (type: string, handler: MessageHandler) => () => void;
}

const WsContext = createContext<WsContextValue>({
  connected: false,
  subscribe: () => () => {},
});

const RECONNECT_DELAY = 3000;

export function WsProvider({ children }: { children: React.ReactNode }) {
  const [connected, setConnected] = useState(false);
  const subscribersRef = useRef<Map<string, Set<MessageHandler>>>(new Map());
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const mountedRef = useRef(true);
  const connectRef = useRef<() => void>(() => {});

  const connect = useCallback(() => {
    if (!mountedRef.current) return;
    const token = getToken();
    if (!token) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${protocol}//${window.location.host}/api/v1/ws?token=${encodeURIComponent(token)}`;

    try {
      const ws = new WebSocket(url);
      wsRef.current = ws;

      ws.onopen = () => {
        if (mountedRef.current) setConnected(true);
      };

      ws.onmessage = (event) => {
        if (!mountedRef.current) return;
        try {
          const msg = JSON.parse(event.data as string) as { type: string; data: unknown };
          subscribersRef.current.get(msg.type)?.forEach((h) => h(msg.data));
        } catch {
          // ignore malformed messages
        }
      };

      ws.onclose = () => {
        if (mountedRef.current) {
          setConnected(false);
          reconnectTimer.current = setTimeout(() => connectRef.current(), RECONNECT_DELAY);
        }
      };

      ws.onerror = () => {
        ws.close();
      };
    } catch {
      if (mountedRef.current) {
        reconnectTimer.current = setTimeout(() => connectRef.current(), RECONNECT_DELAY);
      }
    }
  }, []);

  useEffect(() => {
    connectRef.current = connect;
  }, [connect]);

  useEffect(() => {
    mountedRef.current = true;
    connect();
    return () => {
      mountedRef.current = false;
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
      wsRef.current?.close();
    };
  }, [connect]);

  const subscribe = useCallback((type: string, handler: MessageHandler): (() => void) => {
    if (!subscribersRef.current.has(type)) {
      subscribersRef.current.set(type, new Set());
    }
    subscribersRef.current.get(type)!.add(handler);
    return () => {
      subscribersRef.current.get(type)?.delete(handler);
    };
  }, []);

  return <WsContext.Provider value={{ connected, subscribe }}>{children}</WsContext.Provider>;
}

// eslint-disable-next-line react-refresh/only-export-components
export function useWsSubscribe() {
  return useContext(WsContext);
}
