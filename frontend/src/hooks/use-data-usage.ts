import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type { DataBudgetConfig } from '@shared/index';
import {
  getDataBudget,
  getDataUsage,
  getTrafficHistory,
  resetDataUsage,
  setDataBudget,
} from '@/api/endpoints';

export function useDataUsage() {
  return useQuery({
    queryKey: ['data-usage'],
    queryFn: () => getDataUsage(),
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
    queryFn: ({ signal }) => getTrafficHistory({ signal }),
    staleTime: 5 * 60_000,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
}

export function useDataBudget() {
  return useQuery({
    queryKey: ['data-usage-budget'],
    queryFn: () => getDataBudget(),
  });
}

export function useSetDataBudget() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (cfg: DataBudgetConfig) => setDataBudget(cfg),
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
      resetDataUsage({
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
