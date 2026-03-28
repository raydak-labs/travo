import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type { CaptiveAutoAcceptResult, CaptivePortalStatus } from '@shared/index';

export function useCaptivePortal() {
  return useQuery({
    queryKey: ['captive', 'status'],
    queryFn: () => apiClient.get<CaptivePortalStatus>(API_ROUTES.captive.status),
    refetchInterval: 30_000,
  });
}

export function useCaptiveAutoAccept() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: { portal_url?: string } = {}) =>
      apiClient.post<CaptiveAutoAcceptResult>(API_ROUTES.captive.autoAccept, body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['captive', 'status'] });
    },
  });
}
