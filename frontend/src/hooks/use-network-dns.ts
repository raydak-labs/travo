/**
 * DNS: the resolver configuration, the static host entries, and dynamic DNS.
 */
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { routeWithSegment } from '@/lib/api-url';
import { API_ROUTES } from '@shared/index';
import type {
  DNSConfig,
  DNSEntry,
  DDNSConfig,
  DDNSConfigResponse,
  DDNSStatus,
} from '@shared/index';

export function useDNSConfig() {
  return useQuery({
    queryKey: ['network', 'dns'],
    queryFn: ({ signal }) => apiClient.get<DNSConfig>(API_ROUTES.network.dns, signal),
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

export function useDNSEntries() {
  return useQuery({
    queryKey: ['network', 'dnsEntries'],
    queryFn: ({ signal }) => apiClient.get<DNSEntry[]>(API_ROUTES.network.dnsEntries, signal),
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
      apiClient.del<{ status: string }>(routeWithSegment(API_ROUTES.network.dnsEntries, section)),
    onSuccess: () => {
      toast.success('DNS entry deleted');
      void queryClient.invalidateQueries({ queryKey: ['network', 'dnsEntries'] });
    },
    onError: (error) => {
      toast.error('Failed to delete DNS entry', { description: error.message });
    },
  });
}

export function useDDNSConfig() {
  return useQuery({
    queryKey: ['network', 'ddns'],
    queryFn: ({ signal }) => apiClient.get<DDNSConfigResponse>(API_ROUTES.network.ddns, signal),
  });
}

export function useDDNSStatus() {
  return useQuery({
    queryKey: ['network', 'ddnsStatus'],
    queryFn: ({ signal }) => apiClient.get<DDNSStatus>(API_ROUTES.network.ddnsStatus, signal),
    refetchInterval: 30000,
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
