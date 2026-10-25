import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type { SQMConfig } from '@shared/index';
import { applySQM, getSQMConfig, setSQMConfig } from '@/api/endpoints';

export function useSQMConfig(enabled = true) {
  return useQuery({
    queryKey: ['sqm-config'],
    queryFn: () => getSQMConfig(),
    enabled,
  });
}

export function useSetSQMConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (cfg: SQMConfig) => setSQMConfig(cfg),
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
    mutationFn: () => applySQM(),
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
