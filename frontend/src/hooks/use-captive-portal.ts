import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type { CaptivePortalStatus } from '@shared/index';

export function useCaptivePortal() {
  return useQuery({
    queryKey: ['captive', 'status'],
    queryFn: () => apiClient.get<CaptivePortalStatus>(API_ROUTES.captive.status),
    refetchInterval: 30_000,
  });
}
