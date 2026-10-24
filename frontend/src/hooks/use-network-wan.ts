/**
 * WAN: the uplink configuration, the connection method, the failover chain and
 * the uptime event log.
 */
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type {
  WanConfig,
  WanDetectResult,
  ConnectionMethod,
  FailoverConfig,
  FailoverEvent,
  UptimeEvent,
} from '@shared/index';

export function useWanConfig() {
  return useQuery({
    queryKey: ['network', 'wan'],
    queryFn: ({ signal }) => apiClient.get<WanConfig>(API_ROUTES.network.wan, signal),
  });
}

export function useDetectWanType() {
  return useMutation({
    mutationFn: () => apiClient.get<WanDetectResult>(API_ROUTES.network.wanDetect),
    onError: (error) => {
      toast.error('WAN detection failed', { description: error.message });
    },
  });
}

export function useConnectionMethod() {
  return useQuery({
    queryKey: ['network', 'connectionMethod'],
    queryFn: ({ signal }) =>
      apiClient.get<ConnectionMethod>(API_ROUTES.network.connectionMethod, signal),
    // Cache for 1 minute - connection method doesn't change rapidly
    staleTime: 60000,
  });
}

export function useFailoverConfig() {
  return useQuery({
    queryKey: ['network', 'failover'],
    queryFn: ({ signal }) => apiClient.get<FailoverConfig>(API_ROUTES.network.failover, signal),
  });
}

export function useSetFailoverConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (config: FailoverConfig) =>
      apiClient.put<{ status: string }>(API_ROUTES.network.failover, config),
    onSuccess: () => {
      toast.success('Connection failover updated');
      void queryClient.invalidateQueries({ queryKey: ['network', 'failover'] });
      void queryClient.invalidateQueries({ queryKey: ['network', 'failoverEvents'] });
      void queryClient.invalidateQueries({ queryKey: ['network', 'status'] });
    },
    onError: (error) => {
      toast.error('Failed to update connection failover', { description: error.message });
    },
  });
}

export function useFailoverEvents() {
  return useQuery({
    queryKey: ['network', 'failoverEvents'],
    queryFn: ({ signal }) =>
      apiClient.get<FailoverEvent[]>(API_ROUTES.network.failoverEvents, signal),
    refetchInterval: 15000,
  });
}

export function useUptimeLog() {
  return useQuery({
    queryKey: ['network', 'uptimeLog'],
    queryFn: ({ signal }) => apiClient.get<UptimeEvent[]>(API_ROUTES.network.uptimeLog, signal),
    refetchInterval: 60000,
  });
}
