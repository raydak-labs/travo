import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type { SQMApplyResult, SQMConfig } from '@shared/index';

export function useSQMConfig(enabled = true) {
  return useQuery({
    queryKey: ['sqm-config'],
    queryFn: () => apiClient.get<SQMConfig>(API_ROUTES.sqm.config),
    enabled,
  });
}

export function useSetSQMConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (cfg: SQMConfig) => apiClient.put<{ status: string }>(API_ROUTES.sqm.config, cfg),
    onSuccess: () => {
      toast.success('SQM configuration saved');
      void queryClient.invalidateQueries({ queryKey: ['sqm-config'] });
    },
    onError: (error) => {
      toast.error('Failed to save SQM configuration', { description: error.message });
    },
  });
}

export function useApplySQM() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => apiClient.post<SQMApplyResult>(API_ROUTES.sqm.apply),
    onSuccess: (res) => {
      if (res.ok) {
        toast.success('SQM applied');
      } else {
        toast.error('SQM apply failed', { description: res.error ?? 'Unknown error' });
      }
      void queryClient.invalidateQueries({ queryKey: ['sqm-config'] });
    },
    onError: (error) => {
      toast.error('SQM apply failed', { description: error.message });
    },
  });
}
