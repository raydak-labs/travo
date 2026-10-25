/**
 * DHCP: the pool configuration, the live lease list, and the static
 * reservations.
 */
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type { DHCPConfig } from '@shared/index';
import {
  addDHCPReservation,
  deleteDHCPReservation,
  getDHCPConfig,
  getDHCPLeases,
  getDHCPReservations,
  setDHCPConfig,
} from '@/api/endpoints';

export function useDHCPConfig() {
  return useQuery({
    queryKey: ['network', 'dhcp'],
    queryFn: ({ signal }) => getDHCPConfig({ signal }),
  });
}

export function useDHCPLeases() {
  return useQuery({
    queryKey: ['network', 'dhcpLeases'],
    queryFn: ({ signal }) => getDHCPLeases({ signal }),
    refetchInterval: 30000,
  });
}

export function useSetDHCPConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (config: DHCPConfig) => setDHCPConfig(config),
    onSuccess: () => {
      toast.success('DHCP configuration updated');
      void queryClient.invalidateQueries({ queryKey: ['network', 'dhcp'] });
    },
    onError: (error) => {
      toast.error('Failed to update DHCP config', { description: error.message });
    },
  });
}

export function useDHCPReservations() {
  return useQuery({
    queryKey: ['network', 'dhcpReservations'],
    queryFn: ({ signal }) => getDHCPReservations({ signal }),
  });
}

export function useAddDHCPReservation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (reservation: { name: string; mac: string; ip: string }) =>
      addDHCPReservation(reservation),
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
    mutationFn: (section: string) => deleteDHCPReservation(section),
    onSuccess: () => {
      toast.success('DHCP reservation deleted');
      void queryClient.invalidateQueries({ queryKey: ['network', 'dhcpReservations'] });
    },
    onError: (error) => {
      toast.error('Failed to delete DHCP reservation', { description: error.message });
    },
  });
}
