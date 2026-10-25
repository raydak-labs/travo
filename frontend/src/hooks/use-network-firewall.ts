/**
 * Firewall and the other advanced network tools: zones, port forwards,
 * diagnostics, wake-on-LAN, IPv6 and DNS-over-HTTPS.
 */
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { routeWithSegment } from '@/lib/api-url';
import { API_ROUTES } from '@shared/index';
import type {
  AddPortForwardRequest,
  DiagnosticsRequest,
  DiagnosticsResult,
  DoHConfig,
  FirewallZone,
  IPv6Status,
  PortForwardRule,
  Status,
  WoLRequest,
} from '@shared/index';

export function useFirewallZones() {
  return useQuery({
    queryKey: ['network', 'firewallZones'],
    queryFn: async () => {
      const res = await apiClient.get<{ zones: FirewallZone[] }>(API_ROUTES.network.firewallZones);
      return res.zones;
    },
  });
}

export function usePortForwards() {
  return useQuery({
    queryKey: ['network', 'portForwards'],
    queryFn: async () => {
      const res = await apiClient.get<{ rules: PortForwardRule[] }>(
        API_ROUTES.network.portForwards,
      );
      return res.rules;
    },
  });
}

export function useAddPortForward() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (rule: AddPortForwardRequest) =>
      apiClient.post<Status>(API_ROUTES.network.portForwards, rule),
    onSuccess: () => {
      toast.success('Port forward added');
      void queryClient.invalidateQueries({ queryKey: ['network', 'portForwards'] });
    },
    onError: (error) => {
      toast.error('Failed to add port forward', { description: error.message });
    },
  });
}

export function useDeletePortForward() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.del<Status>(routeWithSegment(API_ROUTES.network.portForwards, id)),
    onSuccess: () => {
      toast.success('Port forward deleted');
      void queryClient.invalidateQueries({ queryKey: ['network', 'portForwards'] });
    },
    onError: (error) => {
      toast.error('Failed to delete port forward', { description: error.message });
    },
  });
}

export function useRunDiagnostics() {
  return useMutation({
    mutationFn: (req: DiagnosticsRequest) =>
      apiClient.post<DiagnosticsResult>(API_ROUTES.network.diagnostics, req),
    onError: (error) => {
      toast.error('Diagnostic failed', { description: error.message });
    },
  });
}

export function useSendWoL() {
  return useMutation({
    mutationFn: (req: WoLRequest) => apiClient.post<Status>(API_ROUTES.network.wol, req),
    onSuccess: () => {
      toast.success('Wake-on-LAN packet sent');
    },
    onError: (error) => {
      toast.error('Failed to send WoL packet', { description: error.message });
    },
  });
}

export function useIPv6Status() {
  return useQuery({
    queryKey: ['network', 'ipv6'],
    queryFn: ({ signal }) => apiClient.get<IPv6Status>(API_ROUTES.network.ipv6, signal),
  });
}

export function useSetIPv6Enabled() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (enabled: boolean) => apiClient.put<Status>(API_ROUTES.network.ipv6, { enabled }),
    onSuccess: () => {
      toast.success('IPv6 setting saved');
      void queryClient.invalidateQueries({ queryKey: ['network', 'ipv6'] });
    },
    onError: (error) => {
      toast.error('Failed to save IPv6 setting', { description: error.message });
    },
  });
}

export function useDoHConfig() {
  return useQuery({
    queryKey: ['network', 'doh'],
    queryFn: ({ signal }) => apiClient.get<DoHConfig>(API_ROUTES.network.doh, signal),
  });
}

export function useSetDoHConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (cfg: DoHConfig) => apiClient.put<Status>(API_ROUTES.network.doh, cfg),
    onSuccess: () => {
      toast.success('DNS-over-HTTPS settings saved');
      void queryClient.invalidateQueries({ queryKey: ['network', 'doh'] });
    },
    onError: (error) => {
      toast.error('Failed to save DoH settings', { description: error.message });
    },
  });
}
