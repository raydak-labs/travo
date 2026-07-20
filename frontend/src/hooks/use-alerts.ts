import { useEffect, useRef } from 'react';
import { useQuery } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type { Alert, AlertsResponse } from '@shared/index';
import { useAlertStore } from '@/stores/alert-store';
import { useWsSubscribe } from '@/lib/ws-context';

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

/**
 * Polling cadence for alert history: live alerts arrive over the WebSocket,
 * so HTTP polling only runs as a fallback while the socket is down.
 * Exported for tests.
 */
export function alertsRefetchInterval(wsConnected: boolean): number | false {
  return wsConnected ? false : 30_000;
}

export function useAlerts() {
  const { alerts, unreadCount, addAlert, setAlerts, markAllRead } = useAlertStore();
  const { connected, subscribe } = useWsSubscribe();
  const wasDisconnectedRef = useRef(false);

  // Fetch history on mount; poll only while the WebSocket is down.
  const { refetch } = useQuery({
    queryKey: ['system', 'alerts'],
    queryFn: async () => {
      const data = await apiClient.get<AlertsResponse>(API_ROUTES.system.alerts);
      setAlerts([...data.alerts]);
      return data;
    },
    refetchInterval: alertsRefetchInterval(connected),
  });

  // Catch up on alerts missed while the socket was down.
  useEffect(() => {
    if (!connected) {
      wasDisconnectedRef.current = true;
    } else if (wasDisconnectedRef.current) {
      wasDisconnectedRef.current = false;
      void refetch();
    }
  }, [connected, refetch]);

  // Subscribe to real-time alerts via the shared WebSocket
  useEffect(() => {
    return subscribe('alert', (data) => {
      const alert = data as Alert;
      addAlert(alert);
      showAlertToast(alert);
    });
  }, [subscribe, addAlert]);

  return { alerts, unreadCount, markAllRead };
}
