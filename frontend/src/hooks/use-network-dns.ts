/**
 * DNS: the resolver configuration, the static host entries, and dynamic DNS.
 */
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type { DNSConfig, DDNSConfig } from '@shared/index';
import {
  addDNSEntry,
  deleteDNSEntry,
  getDDNSConfig,
  getDDNSStatus,
  getDNSConfig,
  getDNSEntries,
  setDDNSConfig,
  setDNSConfig,
} from '@/api/endpoints';

export function useDNSConfig() {
  return useQuery({
    queryKey: ['network', 'dns'],
    queryFn: ({ signal }) => getDNSConfig({ signal }),
  });
}

export function useSetDNSConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (config: DNSConfig) => setDNSConfig(config),
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
    queryFn: ({ signal }) => getDNSEntries({ signal }),
  });
}

export function useAddDNSEntry() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (entry: { name: string; ip: string }) => addDNSEntry(entry),
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
    mutationFn: (section: string) => deleteDNSEntry(section),
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
    queryFn: ({ signal }) => getDDNSConfig({ signal }),
  });
}

export function useDDNSStatus() {
  return useQuery({
    queryKey: ['network', 'ddnsStatus'],
    queryFn: ({ signal }) => getDDNSStatus({ signal }),
    refetchInterval: 30000,
  });
}

export function useSetDDNSConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (config: DDNSConfig) => setDDNSConfig(config),
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
