import { useEffect, useRef, useCallback } from 'react';
import { useQuery } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient, getToken } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type { Alert, AlertsResponse } from '@shared/index';
import { useAlertStore } from '@/stores/alert-store';

const RECONNECT_DELAY = 5000;

const severityLabels: Record<string, string> = {
  info: 'Info',
  warning: 'Warning',
  critical: 'Critical',
};

function showAlertToast(alert: Alert) {
  const label = severityLabels[alert.severity] ?? alert.severity;
  if (alert.severity === 'critical') {
    toast.error(`${label}: ${alert.message}`);
  } else if (alert.severity === 'warning') {
    toast.warning(`${label}: ${alert.message}`);
  } else {
    toast.info(`${label}: ${alert.message}`);
  }
}

export function useAlerts() {
  const { alerts, unreadCount, addAlert, setAlerts, markAllRead } = useAlertStore();
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const mountedRef = useRef(true);
  const connectRef = useRef<() => void>(() => {});

  // Fetch history on mount
  useQuery({
    queryKey: ['system', 'alerts'],
    queryFn: async () => {
      const data = await apiClient.get<AlertsResponse>(API_ROUTES.system.alerts);
      setAlerts([...data.alerts]);
      return data;
    },
    refetchInterval: 30_000,
  });

  const connect = useCallback(() => {
    if (!mountedRef.current) return;
    const token = getToken();
    if (!token) return;

    // Don't create duplicate connections
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${protocol}//${window.location.host}/api/v1/ws?token=${encodeURIComponent(token)}`;

    try {
      const ws = new WebSocket(url);
      wsRef.current = ws;

      ws.onmessage = (event) => {
        if (!mountedRef.current) return;
        try {
          const msg = JSON.parse(event.data) as { type: string; data: Alert };
          if (msg.type === 'alert' && msg.data) {
            addAlert(msg.data);
            showAlertToast(msg.data);
          }
        } catch {
          // Ignore non-alert messages
        }
      };

      ws.onclose = () => {
        if (mountedRef.current) {
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
  }, [addAlert]);

  useEffect(() => {
    connectRef.current = connect;
  }, [connect]);

  useEffect(() => {
    mountedRef.current = true;
    connect();

    return () => {
      mountedRef.current = false;
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
      if (wsRef.current) wsRef.current.close();
    };
  }, [connect]);

  return { alerts, unreadCount, markAllRead };
}
