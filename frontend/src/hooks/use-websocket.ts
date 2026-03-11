import { useEffect, useRef, useCallback, useState } from 'react';
import { getToken } from '@/lib/api-client';
import type { SystemStats } from '@shared/index';

export interface StatsDataPoint {
  timestamp: number;
  cpu: number;
  memoryUsed: number;
  memoryTotal: number;
  rxBytes: number;
  txBytes: number;
}

const MAX_POINTS = 15; // 30 seconds at 2s intervals
const RECONNECT_DELAY = 3000;

/**
 * Connects to the WebSocket endpoint and buffers the last 15 data points
 * of system stats for charting.
 */
export function useWebSocket() {
  const [dataPoints, setDataPoints] = useState<StatsDataPoint[]>([]);
  const [connected, setConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const mountedRef = useRef(true);

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
        if (mountedRef.current) {
          setConnected(true);
        }
      };

      ws.onmessage = (event) => {
        if (!mountedRef.current) return;
        try {
          const msg = JSON.parse(event.data) as {
            type: string;
            data: SystemStats;
          };
          if (msg.type === 'system_stats' && msg.data) {
            const point: StatsDataPoint = {
              timestamp: Date.now(),
              cpu: msg.data.cpu.usage_percent,
              memoryUsed: msg.data.memory.used_bytes,
              memoryTotal: msg.data.memory.total_bytes,
              rxBytes: 0,
              txBytes: 0,
            };
            setDataPoints((prev) => {
              const next = [...prev, point];
              return next.length > MAX_POINTS ? next.slice(next.length - MAX_POINTS) : next;
            });
          }
        } catch {
          // Ignore malformed messages
        }
      };

      ws.onclose = () => {
        if (mountedRef.current) {
          setConnected(false);
          setDataPoints([]);
          reconnectTimer.current = setTimeout(connect, RECONNECT_DELAY);
        }
      };

      ws.onerror = () => {
        ws.close();
      };
    } catch {
      // WebSocket construction failed, retry later
      if (mountedRef.current) {
        reconnectTimer.current = setTimeout(connect, RECONNECT_DELAY);
      }
    }
  }, []);

  useEffect(() => {
    mountedRef.current = true;
    connect();

    return () => {
      mountedRef.current = false;
      if (reconnectTimer.current) {
        clearTimeout(reconnectTimer.current);
      }
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, [connect]);

  return { dataPoints, connected };
}
