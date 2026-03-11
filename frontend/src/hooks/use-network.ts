import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type {
  NetworkStatus,
  WanConfig,
  Client,
  DHCPConfig,
  DNSConfig,
  DHCPLease,
  SetAliasRequest,
  DNSEntry,
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
