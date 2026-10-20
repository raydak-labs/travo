/**
 * DHCP: the pool configuration, the live lease list, and the static
 * reservations.
 */
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { routeWithSegment } from '@/lib/api-url';
import { API_ROUTES } from '@shared/index';
import type { DHCPConfig, DHCPLease, DHCPReservation } from '@shared/index';

export function useDHCPConfig() {
  return useQuery({
    queryKey: ['network', 'dhcp'],
    queryFn: ({ signal }) => apiClient.get<DHCPConfig>(API_ROUTES.network.dhcp, signal),
  });
}

export function useDHCPLeases() {
  return useQuery({
    queryKey: ['network', 'dhcpLeases'],
    queryFn: ({ signal }) => apiClient.get<DHCPLease[]>(API_ROUTES.network.dhcpLeases, signal),
    refetchInterval: 30000,
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

export function useDHCPReservations() {
  return useQuery({
    queryKey: ['network', 'dhcpReservations'],
    queryFn: ({ signal }) =>
      apiClient.get<DHCPReservation[]>(API_ROUTES.network.dhcpReservations, signal),
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
      apiClient.del<{ status: string }>(
        routeWithSegment(API_ROUTES.network.dhcpReservations, section),
      ),
    onSuccess: () => {
      toast.success('DHCP reservation deleted');
      void queryClient.invalidateQueries({ queryKey: ['network', 'dhcpReservations'] });
    },
    onError: (error) => {
      toast.error('Failed to delete DHCP reservation', { description: error.message });
    },
  });
}
