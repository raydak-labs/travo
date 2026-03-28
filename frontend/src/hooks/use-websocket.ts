import { useEffect, useState } from 'react';
import type { SystemStats } from '@shared/index';
import { useWsSubscribe } from '@/lib/ws-context';

export interface StatsDataPoint {
  timestamp: number;
  cpu: number;
  memoryUsed: number;
  memoryTotal: number;
  rxBytes: number;
  txBytes: number;
}

export interface InterfaceDataPoint {
  timestamp: number;
  rxBytes: number;
  txBytes: number;
}

const MAX_POINTS = 15; // 30 seconds at 2s intervals

/**
 * Subscribes to system_stats WebSocket messages via WsContext and buffers
 * the last 15 data points for charting. Public API is unchanged from the
 * previous self-managing version.
 */
export function useWebSocket() {
  const { connected, subscribe } = useWsSubscribe();
  const [dataPoints, setDataPoints] = useState<StatsDataPoint[]>([]);
  const [interfaceDataPoints, setInterfaceDataPoints] = useState<
    Record<string, InterfaceDataPoint[]>
  >({});

  // Clear stale chart data when the connection drops.
  useEffect(() => {
    if (!connected) {
      setDataPoints([]);
      setInterfaceDataPoints({});
    }
  }, [connected]);

  useEffect(() => {
    return subscribe('system_stats', (raw) => {
      const msg = raw as SystemStats;
      if (!msg) return;

      const net = msg.network?.[0];
      const point: StatsDataPoint = {
        timestamp: Date.now(),
        cpu: msg.cpu.usage_percent,
        memoryUsed: msg.memory.used_bytes,
        memoryTotal: msg.memory.total_bytes,
        rxBytes: net?.rx_bytes ?? 0,
        txBytes: net?.tx_bytes ?? 0,
      };
      setDataPoints((prev) => {
        const next = [...prev, point];
        return next.length > MAX_POINTS ? next.slice(next.length - MAX_POINTS) : next;
      });

      if (msg.network) {
        const ts = Date.now();
        setInterfaceDataPoints((prev) => {
          const updated = { ...prev };
          for (const iface of msg.network) {
            const key = iface.interface;
            const ifPoint: InterfaceDataPoint = {
              timestamp: ts,
              rxBytes: iface.rx_bytes,
              txBytes: iface.tx_bytes,
            };
            const existing = updated[key] ?? [];
            const next = [...existing, ifPoint];
            updated[key] =
              next.length > MAX_POINTS ? next.slice(next.length - MAX_POINTS) : next;
          }
          return updated;
        });
      }
    });
  }, [subscribe]);

  return { dataPoints, interfaceDataPoints, connected };
}
