import { useQuery, useMutation } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type { SystemInfo, SystemStats } from '@shared/index';

export function useSystemInfo() {
  return useQuery({
    queryKey: ['system', 'info'],
    queryFn: () => apiClient.get<SystemInfo>(API_ROUTES.system.info),
  });
}

export function useSystemStats() {
  return useQuery({
    queryKey: ['system', 'stats'],
    queryFn: () => apiClient.get<SystemStats>(API_ROUTES.system.stats),
    refetchInterval: 5_000,
  });
}

export function useReboot() {
  return useMutation({
    mutationFn: () => apiClient.post<{ status: string }>(API_ROUTES.system.reboot),
    onSuccess: () => {
      toast.success('Reboot initiated', { description: 'The device is restarting…' });
    },
    onError: (error) => {
      toast.error('Failed to reboot', { description: error.message });
    },
  });
}
