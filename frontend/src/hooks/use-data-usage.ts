import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type { DataUsageStatus, DataBudgetConfig, TrafficHistoryResponse } from '@shared/index';

export function useDataUsage() {
  return useQuery({
    queryKey: ['data-usage'],
    queryFn: () => apiClient.get<DataUsageStatus>(API_ROUTES.network.dataUsage),
    refetchInterval: 60_000, // refresh every minute
  });
}

/**
 * The server's bounded per-interface traffic history, fetched once per page
 * load. The chart owns the live tail over WebSocket; this exists so a freshly
 * loaded dashboard has something to draw instead of an empty axis.
 */
export function useTrafficHistory() {
  return useQuery({
    queryKey: ['network', 'traffic-history'],
    queryFn: ({ signal }) =>
      apiClient.get<TrafficHistoryResponse>(API_ROUTES.network.trafficHistory, signal),
    staleTime: 5 * 60_000,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
}

export function useDataBudget() {
  return useQuery({
    queryKey: ['data-usage-budget'],
    queryFn: () => apiClient.get<DataBudgetConfig>(API_ROUTES.network.dataUsageBudget),
  });
}

export function useSetDataBudget() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (cfg: DataBudgetConfig) =>
      apiClient.put<{ status: string }>(API_ROUTES.network.dataUsageBudget, cfg),
    onSuccess: () => {
      toast.success('Data budget saved');
      void queryClient.invalidateQueries({ queryKey: ['data-usage-budget'] });
    },
    onError: (error: Error) => {
      toast.error('Failed to save data budget', { description: error.message });
    },
  });
}

export function useResetDataUsage() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (ifaceName: string) =>
      apiClient.post<{ status: string }>(API_ROUTES.network.dataUsageReset, {
        interface: ifaceName,
      }),
    onSuccess: () => {
      toast.success('Usage counters reset');
      void queryClient.invalidateQueries({ queryKey: ['data-usage'] });
    },
    onError: (error: Error) => {
      toast.error('Failed to reset counters', { description: error.message });
    },
  });
}
