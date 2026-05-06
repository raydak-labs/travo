import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';

export interface StatsHistoryPoint {
  time: number;
  cpu: number;
  memory: number;
  rx_bytes: number;
  tx_bytes: number;
}

export function useStatsHistory(since?: number) {
  const url = since
    ? `${API_ROUTES.system.statsHistory}?since=${since}`
    : API_ROUTES.system.statsHistory;

  return useQuery({
    queryKey: ['system', 'stats', 'history', since],
    queryFn: () => apiClient.get<StatsHistoryPoint[]>(url),
    refetchInterval: 30_000,
  });
}
