/**
 * WAN: the uplink configuration, the connection method, the failover chain and
 * the uptime event log.
 */
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type { FailoverConfig } from '@shared/index';
import {
  detectWanType,
  getConnectionMethod,
  getFailoverConfig,
  getFailoverEvents,
  getUptimeLog,
  getWanConfig,
  setFailoverConfig,
} from '@/api/endpoints';

export function useWanConfig() {
  return useQuery({
    queryKey: ['network', 'wan'],
    queryFn: ({ signal }) => getWanConfig({ signal }),
  });
}

export function useDetectWanType() {
  return useMutation({
    mutationFn: () => detectWanType(),
    onError: (error) => {
      toast.error('WAN detection failed', { description: error.message });
    },
  });
}

export function useConnectionMethod() {
  return useQuery({
    queryKey: ['network', 'connectionMethod'],
    queryFn: ({ signal }) => getConnectionMethod({ signal }),
    // Cache for 1 minute - connection method doesn't change rapidly
    staleTime: 60000,
  });
}

export function useFailoverConfig() {
  return useQuery({
    queryKey: ['network', 'failover'],
    queryFn: ({ signal }) => getFailoverConfig({ signal }),
  });
}

export function useSetFailoverConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (config: FailoverConfig) => setFailoverConfig(config),
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
    queryFn: ({ signal }) => getFailoverEvents({ signal }),
    refetchInterval: 15000,
  });
}

export function useUptimeLog() {
  return useQuery({
    queryKey: ['network', 'uptimeLog'],
    queryFn: ({ signal }) => getUptimeLog({ signal }),
    refetchInterval: 60000,
  });
}
