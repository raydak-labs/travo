import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type { NetworkStatus, WanConfig, Client } from '@shared/index';

export function useNetworkStatus() {
  return useQuery({
    queryKey: ['network', 'status'],
    queryFn: () => apiClient.get<NetworkStatus>(API_ROUTES.network.status),
  });
}

export function useWanConfig() {
  return useQuery({
    queryKey: ['network', 'wan'],
    queryFn: () => apiClient.get<WanConfig>(API_ROUTES.network.wan),
  });
}

export function useSetWanConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (config: WanConfig) =>
      apiClient.put<{ success: boolean }>(API_ROUTES.network.wan, config),
    onSuccess: () => {
      toast.success('WAN configuration updated');
      void queryClient.invalidateQueries({ queryKey: ['network'] });
    },
    onError: (error) => {
      toast.error('Failed to update WAN config', { description: error.message });
    },
  });
}

export function useClients() {
  return useQuery({
    queryKey: ['network', 'clients'],
    queryFn: () => apiClient.get<Client[]>(API_ROUTES.network.clients),
  });
}
