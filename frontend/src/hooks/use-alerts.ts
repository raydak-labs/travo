import { useEffect } from 'react';
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

export function useAlerts() {
  const { alerts, unreadCount, addAlert, setAlerts, markAllRead } = useAlertStore();
  const { subscribe } = useWsSubscribe();

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
