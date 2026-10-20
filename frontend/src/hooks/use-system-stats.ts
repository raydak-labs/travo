/**
 * Read-only system telemetry: device info, live stats (fed by the WebSocket
 * hub), the system and kernel logs, and the speed test.
 */
import { useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { useWsSubscribe } from '@/lib/ws-context';
import { useRefetchOnWsReconnect } from '@/hooks/use-refetch-on-ws-reconnect';
import { API_ROUTES } from '@shared/index';
import type { SystemInfo, SystemStats, LogResponse, SpeedTestResult } from '@shared/index';

export function useSystemInfo() {
  return useQuery({
    queryKey: ['system', 'info'],
    queryFn: () => apiClient.get<SystemInfo>(API_ROUTES.system.info),
  });
}

/**
 * Polling cadence for system stats: the WebSocket hub already pushes
 * system_stats every 2s, so HTTP polling is only a fallback while the
 * socket is down. Exported for tests.
 */
export function statsRefetchInterval(wsConnected: boolean): number | false {
  return wsConnected ? false : 5_000;
}

export function useSystemStats() {
  const { connected, subscribe } = useWsSubscribe();
  const queryClient = useQueryClient();
  // Polling stops while connected, so a reconnect has to re-request whatever
  // the socket missed while it was down.
  useRefetchOnWsReconnect([['system', 'stats']]);

  // Feed WebSocket pushes straight into the query cache — every consumer of
  // this hook sees live data without duplicate HTTP requests to the router.
  useEffect(() => {
    return subscribe('system_stats', (raw) => {
      queryClient.setQueryData(['system', 'stats'], raw as SystemStats);
    });
  }, [subscribe, queryClient]);

  return useQuery({
    queryKey: ['system', 'stats'],
    queryFn: () => apiClient.get<SystemStats>(API_ROUTES.system.stats),
    refetchInterval: statsRefetchInterval(connected),
  });
}

/**
 * `enabled` exists so the Logs page can fetch only the visible tab. Both log
 * queries used to be always-on, so opening the page ran `logread` *and*
 * `dmesg` and switching tabs refetched the one being left.
 */
export function useSystemLogs(service?: string, level?: string, enabled = true) {
  return useQuery({
    queryKey: ['system', 'logs', service ?? '', level ?? ''],
    enabled,
    queryFn: ({ signal }) => {
      const params = new URLSearchParams();
      if (service) params.set('service', service);
      if (level) params.set('level', level);
      const qs = params.toString();
      const url = qs ? `${API_ROUTES.system.logs}?${qs}` : API_ROUTES.system.logs;
      return apiClient.get<LogResponse>(url, signal);
    },
  });
}

export function useKernelLogs(enabled = true) {
  return useQuery({
    queryKey: ['system', 'logs', 'kernel'],
    enabled,
    queryFn: ({ signal }) => apiClient.get<LogResponse>(API_ROUTES.system.kernelLogs, signal),
  });
}

export function useRunSpeedTest() {
  return useMutation({
    mutationFn: () => apiClient.post<SpeedTestResult>(API_ROUTES.system.speedTest),
    onError: (error) => {
      toast.error('Speed test failed', { description: error.message });
    },
  });
}
