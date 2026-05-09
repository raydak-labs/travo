import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type { CaptiveAutoAcceptResult, CaptivePortalStatus } from '@shared/index';

export function useCaptivePortal() {
  const query = useQuery({
    queryKey: ['captive', 'status'],
    queryFn: () => apiClient.get<CaptivePortalStatus>(API_ROUTES.captive.status),
    // Poll faster when we know there's no internet (likely captive portal scenario).
    // Once internet is confirmed, back off to 30s to avoid hammering the probe endpoint.
    refetchInterval: (query) => {
      const data = query.state.data;
      if (!data || !data.can_reach_internet) {
        return 5_000;
      }
      return 30_000;
    },
  });
  return query;
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

export function useCaptiveDNSBypass() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      apiClient.post<{ ok: boolean; message: string }>(API_ROUTES.captive.dnsBypass, {}),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['captive', 'status'] });
    },
  });
}

export function useCaptiveDNSRestore() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      apiClient.post<{ ok: boolean; message: string }>(API_ROUTES.captive.dnsRestore, {}),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['captive', 'status'] });
    },
  });
}
