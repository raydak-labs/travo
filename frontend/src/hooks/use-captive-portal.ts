import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  captiveAutoAccept,
  captiveDNSBypass,
  captiveDNSRestore,
  getAdGuardDNSMode,
  getCaptiveStatus,
} from '@/api/endpoints';

export function useCaptivePortal() {
  const query = useQuery({
    queryKey: ['captive', 'status'],
    queryFn: () => getCaptiveStatus(),
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
    mutationFn: (body: { portal_url?: string } = {}) => captiveAutoAccept(body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['captive', 'status'] });
    },
  });
}

export function useCaptiveDNSBypass() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => captiveDNSBypass(),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['captive', 'status'] });
    },
  });
}

export function useCaptiveDNSRestore() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => captiveDNSRestore(),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['captive', 'status'] });
    },
  });
}

export function useDNSMode() {
  return useQuery({
    queryKey: ['adguard', 'dns-mode'],
    queryFn: () => getAdGuardDNSMode(),
    refetchInterval: 30000,
  });
}
