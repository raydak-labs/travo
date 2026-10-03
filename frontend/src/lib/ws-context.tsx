import {
  type ReactNode,
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from 'react';
import { getToken, TOKEN_CHANGE_EVENT } from './api-client';

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

export function WsProvider({ children }: { children: ReactNode }) {
  const [connected, setConnected] = useState(false);
  // Tracked in state so a login (or logout) after mount re-runs the socket
  // effect below: the token can appear long after the provider is mounted,
  // because the provider wraps /login as well.
  const [token, setToken] = useState<string | null>(() => getToken());
  const subscribersRef = useRef<Map<string, Set<MessageHandler>>>(new Map());
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const mountedRef = useRef(true);
  const connectRef = useRef<() => void>(() => {});

  useEffect(() => {
    const syncToken = () => setToken(getToken());
    // `setToken`/`clearToken` notify in-tab; `storage` covers other tabs.
    window.addEventListener(TOKEN_CHANGE_EVENT, syncToken);
    window.addEventListener('storage', syncToken);
    // A tab restored from bfcache may have missed the event.
    syncToken();
    return () => {
      window.removeEventListener(TOKEN_CHANGE_EVENT, syncToken);
      window.removeEventListener('storage', syncToken);
    };
  }, []);

  /** Closes the current socket (if any) and cancels a pending reconnect. */
  const closeSocket = useCallback(() => {
    if (reconnectTimer.current) {
      clearTimeout(reconnectTimer.current);
      reconnectTimer.current = null;
    }
    const ws = wsRef.current;
    wsRef.current = null;
    if (ws) {
      // Detach handlers first: a close() from an obsolete socket must never
      // flip `connected` or schedule another connection.
      ws.onopen = null;
      ws.onmessage = null;
      ws.onclose = null;
      ws.onerror = null;
      try {
        ws.close();
      } catch {
        // already closing / closed
      }
    }
  }, []);

  const teardown = useCallback(() => {
    closeSocket();
    setConnected(false);
  }, [closeSocket]);

  const scheduleReconnect = useCallback(() => {
    if (!mountedRef.current) return;
    if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
    reconnectTimer.current = setTimeout(() => {
      reconnectTimer.current = null;
      connectRef.current();
    }, RECONNECT_DELAY);
  }, []);

  const connect = useCallback(() => {
    if (!mountedRef.current) return;
    const authToken = getToken();
    if (!authToken) {
      // No session: stay disconnected. The token effect reconnects as soon as
      // a login stores one.
      setConnected(false);
      return;
    }
    // Never run two sockets at once.
    teardown();

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${protocol}//${window.location.host}/api/v1/ws?token=${encodeURIComponent(authToken)}`;

    try {
      const ws = new WebSocket(url);
      wsRef.current = ws;
      const isCurrent = () => wsRef.current === ws;

      ws.onopen = () => {
        if (mountedRef.current && isCurrent()) setConnected(true);
      };

      ws.onmessage = (event) => {
        if (!mountedRef.current || !isCurrent()) return;
        try {
          const msg = JSON.parse(event.data as string) as { type: string; data: unknown };
          subscribersRef.current.get(msg.type)?.forEach((h) => h(msg.data));
        } catch {
          // ignore malformed messages
        }
      };

      ws.onclose = () => {
        if (!mountedRef.current || !isCurrent()) return;
        setConnected(false);
        scheduleReconnect();
      };

      ws.onerror = () => {
        if (!isCurrent()) return;
        try {
          ws.close();
        } catch {
          // ignore
        }
      };
    } catch {
      scheduleReconnect();
    }
  }, [scheduleReconnect, teardown]);

  useEffect(() => {
    connectRef.current = connect;
  }, [connect]);

  useEffect(() => {
    mountedRef.current = true;
    if (token) {
      connectRef.current();
    } else {
      // Logged out (or never logged in): drop any socket left over. `connected`
      // is already false here — the cleanup below is what clears it.
      closeSocket();
    }
    return () => {
      mountedRef.current = false;
      teardown();
    };
  }, [token, teardown, closeSocket]);

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

// eslint-disable-next-line react/only-export-components
export function useWsSubscribe() {
  return useContext(WsContext);
}
