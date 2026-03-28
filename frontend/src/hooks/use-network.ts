import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type {
  NetworkStatus,
  WanConfig,
  WanDetectResult,
  Client,
  DHCPConfig,
  DNSConfig,
  DHCPLease,
  SetAliasRequest,
  DNSEntry,
  DHCPReservation,
  DDNSConfig,
  DDNSStatus,
  UptimeEvent,
  FirewallZone,
  PortForwardRule,
  AddPortForwardRequest,
  WoLRequest,
  DoHConfig,
  IPv6Status,
  DiagnosticsRequest,
  DiagnosticsResult,
} from '@shared/index';

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

export function useDHCPConfig() {
  return useQuery({
    queryKey: ['network', 'dhcp'],
    queryFn: () => apiClient.get<DHCPConfig>(API_ROUTES.network.dhcp),
  });
}

export function useDHCPLeases() {
  return useQuery({
    queryKey: ['network', 'dhcpLeases'],
    queryFn: () => apiClient.get<DHCPLease[]>(API_ROUTES.network.dhcpLeases),
    refetchInterval: 30000,
  });
}

export function useDNSConfig() {
  return useQuery({
    queryKey: ['network', 'dns'],
    queryFn: () => apiClient.get<DNSConfig>(API_ROUTES.network.dns),
  });
}

export function useSetDNSConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (config: DNSConfig) =>
      apiClient.put<{ status: string }>(API_ROUTES.network.dns, config),
    onSuccess: () => {
      toast.success('DNS configuration updated');
      void queryClient.invalidateQueries({ queryKey: ['network', 'dns'] });
    },
    onError: (error) => {
      toast.error('Failed to update DNS config', { description: error.message });
    },
  });
}

export function useSetDHCPConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (config: DHCPConfig) =>
      apiClient.put<{ status: string }>(API_ROUTES.network.dhcp, config),
    onSuccess: () => {
      toast.success('DHCP configuration updated');
      void queryClient.invalidateQueries({ queryKey: ['network', 'dhcp'] });
    },
    onError: (error) => {
      toast.error('Failed to update DHCP config', { description: error.message });
    },
  });
}

export function useSetClientAlias() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: SetAliasRequest) =>
      apiClient.put<{ status: string }>(API_ROUTES.network.clientAlias, data),
    onSuccess: () => {
      toast.success('Client alias updated');
      void queryClient.invalidateQueries({ queryKey: ['network'] });
    },
    onError: (error) => {
      toast.error('Failed to update alias', { description: error.message });
    },
  });
}

export function useDNSEntries() {
  return useQuery({
    queryKey: ['network', 'dnsEntries'],
    queryFn: () => apiClient.get<DNSEntry[]>(API_ROUTES.network.dnsEntries),
  });
}

export function useAddDNSEntry() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (entry: { name: string; ip: string }) =>
      apiClient.post<{ status: string }>(API_ROUTES.network.dnsEntries, entry),
    onSuccess: () => {
      toast.success('DNS entry added');
      void queryClient.invalidateQueries({ queryKey: ['network', 'dnsEntries'] });
    },
    onError: (error) => {
      toast.error('Failed to add DNS entry', { description: error.message });
    },
  });
}

export function useDeleteDNSEntry() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (section: string) =>
      apiClient.del<{ status: string }>(`${API_ROUTES.network.dnsEntries}/${section}`),
    onSuccess: () => {
      toast.success('DNS entry deleted');
      void queryClient.invalidateQueries({ queryKey: ['network', 'dnsEntries'] });
    },
    onError: (error) => {
      toast.error('Failed to delete DNS entry', { description: error.message });
    },
  });
}

export function useDHCPReservations() {
  return useQuery({
    queryKey: ['network', 'dhcpReservations'],
    queryFn: () => apiClient.get<DHCPReservation[]>(API_ROUTES.network.dhcpReservations),
  });
}

export function useAddDHCPReservation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (reservation: { name: string; mac: string; ip: string }) =>
      apiClient.post<{ status: string }>(API_ROUTES.network.dhcpReservations, reservation),
    onSuccess: () => {
      toast.success('DHCP reservation added');
      void queryClient.invalidateQueries({ queryKey: ['network', 'dhcpReservations'] });
    },
    onError: (error) => {
      toast.error('Failed to add DHCP reservation', { description: error.message });
    },
  });
}

export function useDeleteDHCPReservation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (section: string) =>
      apiClient.del<{ status: string }>(`${API_ROUTES.network.dhcpReservations}/${section}`),
    onSuccess: () => {
      toast.success('DHCP reservation deleted');
      void queryClient.invalidateQueries({ queryKey: ['network', 'dhcpReservations'] });
    },
    onError: (error) => {
      toast.error('Failed to delete DHCP reservation', { description: error.message });
    },
  });
}

export function useKickClient() {
  return useMutation({
    mutationFn: (mac: string) =>
      apiClient.post<{ status: string }>(API_ROUTES.network.clientKick, { mac }),
    onSuccess: () => {
      toast.success('Client disconnected');
    },
    onError: (error) => {
      toast.error('Failed to kick client', { description: error.message });
    },
  });
}

export function useBlockClient() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (mac: string) =>
      apiClient.post<{ status: string }>(API_ROUTES.network.clientBlock, { mac }),
    onSuccess: () => {
      toast.success('Client blocked');
      void queryClient.invalidateQueries({ queryKey: ['network', 'blockedClients'] });
    },
    onError: (error) => {
      toast.error('Failed to block client', { description: error.message });
    },
  });
}

export function useUnblockClient() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (mac: string) =>
      apiClient.post<{ status: string }>(API_ROUTES.network.clientUnblock, { mac }),
    onSuccess: () => {
      toast.success('Client unblocked');
      void queryClient.invalidateQueries({ queryKey: ['network', 'blockedClients'] });
    },
    onError: (error) => {
      toast.error('Failed to unblock client', { description: error.message });
    },
  });
}

export function useBlockedClients() {
  return useQuery({
    queryKey: ['network', 'blockedClients'],
    queryFn: () => apiClient.get<string[]>(API_ROUTES.network.clientBlocked),
  });
}

export function useSetInterfaceState() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ name, up }: { name: string; up: boolean }) =>
      apiClient.post<{ status: string }>(API_ROUTES.network.interfaceState.replace(':name', name), {
        up,
      }),
    onSuccess: (_data, variables) => {
      toast.success(`Interface ${variables.name} ${variables.up ? 'brought up' : 'brought down'}`);
      void queryClient.invalidateQueries({ queryKey: ['network'] });
    },
    onError: (error) => {
      toast.error('Failed to change interface state', { description: error.message });
    },
  });
}

export function useDetectWanType() {
  return useMutation({
    mutationFn: () => apiClient.get<WanDetectResult>(API_ROUTES.network.wanDetect),
    onError: (error) => {
      toast.error('WAN detection failed', { description: error.message });
    },
  });
}

export function useDDNSConfig() {
  return useQuery({
    queryKey: ['network', 'ddns'],
    queryFn: () => apiClient.get<DDNSConfig>(API_ROUTES.network.ddns),
  });
}

export function useDDNSStatus() {
  return useQuery({
    queryKey: ['network', 'ddnsStatus'],
    queryFn: () => apiClient.get<DDNSStatus>(API_ROUTES.network.ddnsStatus),
    refetchInterval: 30000,
  });
}

export function useUptimeLog() {
  return useQuery({
    queryKey: ['network', 'uptimeLog'],
    queryFn: () => apiClient.get<UptimeEvent[]>(API_ROUTES.network.uptimeLog),
    refetchInterval: 60000,
  });
}

export function useSetDDNSConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (config: DDNSConfig) =>
      apiClient.put<{ status: string }>(API_ROUTES.network.ddns, config),
    onSuccess: () => {
      toast.success('DDNS configuration updated');
      void queryClient.invalidateQueries({ queryKey: ['network', 'ddns'] });
      void queryClient.invalidateQueries({ queryKey: ['network', 'ddnsStatus'] });
    },
    onError: (error) => {
      toast.error('Failed to update DDNS config', { description: error.message });
    },
  });
}

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
      const res = await apiClient.get<{ rules: PortForwardRule[] }>(API_ROUTES.network.portForwards);
      return res.rules;
    },
  });
}

export function useAddPortForward() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (rule: AddPortForwardRequest) =>
      apiClient.post<{ ok: boolean }>(API_ROUTES.network.portForwards, rule),
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
      apiClient.del<{ ok: boolean }>(`${API_ROUTES.network.portForwards}/${id}`),
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

export function useDoHConfig() {
  return useQuery({
    queryKey: ['network', 'doh'],
    queryFn: () => apiClient.get<DoHConfig>(API_ROUTES.network.doh),
  });
}

export function useSetDoHConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (cfg: DoHConfig) =>
      apiClient.put<{ ok: boolean }>(API_ROUTES.network.doh, cfg),
    onSuccess: () => {
      toast.success('DNS-over-HTTPS settings saved');
      void queryClient.invalidateQueries({ queryKey: ['network', 'doh'] });
    },
    onError: (error) => {
      toast.error('Failed to save DoH settings', { description: error.message });
    },
  });
}

export function useIPv6Status() {
  return useQuery({
    queryKey: ['network', 'ipv6'],
    queryFn: () => apiClient.get<IPv6Status>(API_ROUTES.network.ipv6),
  });
}

export function useSetIPv6Enabled() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (enabled: boolean) =>
      apiClient.put<{ ok: boolean }>(API_ROUTES.network.ipv6, { enabled }),
    onSuccess: () => {
      toast.success('IPv6 setting saved');
      void queryClient.invalidateQueries({ queryKey: ['network', 'ipv6'] });
    },
    onError: (error) => {
      toast.error('Failed to save IPv6 setting', { description: error.message });
    },
  });
}

export function useSendWoL() {
  return useMutation({
    mutationFn: (req: WoLRequest) =>
      apiClient.post<{ ok: boolean }>(API_ROUTES.network.wol, req),
    onSuccess: () => {
      toast.success('Wake-on-LAN packet sent');
    },
    onError: (error) => {
      toast.error('Failed to send WoL packet', { description: error.message });
    },
  });
}

// Aliases used by legacy component files
export const usePortForwardRules = usePortForwards;
export const useAddPortForwardRule = useAddPortForward;
export const useDeletePortForwardRule = useDeletePortForward;
export const useDoHStatus = useDoHConfig;
export const useSetDoHEnabled = useSetDoHConfig;
