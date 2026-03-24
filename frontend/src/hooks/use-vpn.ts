import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type {
  VpnStatus,
  WireguardConfig,
  TailscaleStatus,
  WireGuardStatus,
  WireGuardProfile,
  KillSwitchStatus,
  DNSLeakResult,
  VPNVerifyResult,
} from '@shared/index';

export function useVpnStatus() {
  return useQuery({
    queryKey: ['vpn', 'status'],
    queryFn: () => apiClient.get<VpnStatus[]>(API_ROUTES.vpn.status),
  });
}

export function useWireguardConfig() {
  return useQuery({
    queryKey: ['vpn', 'wireguard'],
    queryFn: () => apiClient.get<WireguardConfig>(API_ROUTES.vpn.wireguard.config),
  });
}

export function useWireguardStatus() {
  return useQuery({
    queryKey: ['vpn', 'wireguard', 'status'],
    queryFn: () => apiClient.get<WireGuardStatus>(API_ROUTES.vpn.wireguard.status),
    refetchInterval: 10000, // refresh every 10 seconds for live stats
  });
}

export function useSetWireguardConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (config: WireguardConfig) =>
      apiClient.put<{ success: boolean }>(API_ROUTES.vpn.wireguard.config, config),
    onSuccess: () => {
      toast.success('WireGuard configuration updated');
      void queryClient.invalidateQueries({ queryKey: ['vpn'] });
    },
    onError: (error) => {
      toast.error('Failed to update WireGuard config', { description: error.message });
    },
  });
}

export function useToggleWireguard() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (enable: boolean) =>
      apiClient.post<{ success: boolean }>(API_ROUTES.vpn.wireguard.toggle, { enable }),
    onSuccess: (_data, enable) => {
      toast.success(`WireGuard ${enable ? 'enabled' : 'disabled'}`);
      void queryClient.invalidateQueries({ queryKey: ['vpn'] });
    },
    onError: (error) => {
      toast.error('Failed to toggle WireGuard', { description: error.message });
    },
  });
}

export function useTailscaleStatus() {
  return useQuery({
    queryKey: ['vpn', 'tailscale'],
    queryFn: () => apiClient.get<TailscaleStatus>(API_ROUTES.vpn.tailscale.status),
  });
}

export function useToggleTailscale() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (enable: boolean) =>
      apiClient.post<{ success: boolean }>(API_ROUTES.vpn.tailscale.toggle, { enable }),
    onSuccess: (_data, enable) => {
      toast.success(`Tailscale ${enable ? 'enabled' : 'disabled'}`);
      void queryClient.invalidateQueries({ queryKey: ['vpn'] });
    },
    onError: (error) => {
      toast.error('Failed to toggle Tailscale', { description: error.message });
    },
  });
}

export function useWireguardProfiles() {
  return useQuery({
    queryKey: ['vpn', 'wireguard', 'profiles'],
    queryFn: () => apiClient.get<WireGuardProfile[]>(API_ROUTES.vpn.wireguard.profiles),
  });
}

export function useAddWireguardProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: { name: string; config: string }) =>
      apiClient.post<WireGuardProfile>(API_ROUTES.vpn.wireguard.profiles, data),
    onSuccess: () => {
      toast.success('WireGuard profile saved');
      void queryClient.invalidateQueries({ queryKey: ['vpn', 'wireguard', 'profiles'] });
    },
    onError: (error) => {
      toast.error('Failed to save profile', { description: error.message });
    },
  });
}

export function useDeleteWireguardProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.del<{ status: string }>(`${API_ROUTES.vpn.wireguard.profiles}/${id}`),
    onSuccess: () => {
      toast.success('WireGuard profile deleted');
      void queryClient.invalidateQueries({ queryKey: ['vpn', 'wireguard', 'profiles'] });
    },
    onError: (error) => {
      toast.error('Failed to delete profile', { description: error.message });
    },
  });
}

export function useActivateWireguardProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      apiClient.post<{ status: string }>(`${API_ROUTES.vpn.wireguard.profiles}/${id}/activate`),
    onSuccess: () => {
      toast.success('WireGuard profile activated');
      void queryClient.invalidateQueries({ queryKey: ['vpn'] });
    },
    onError: (error) => {
      toast.error('Failed to activate profile', { description: error.message });
    },
  });
}

export function useKillSwitch() {
  return useQuery({
    queryKey: ['vpn', 'killswitch'],
    queryFn: () => apiClient.get<KillSwitchStatus>(API_ROUTES.vpn.killswitch),
  });
}

export function useDNSLeakTest() {
  return useMutation({
    mutationFn: () => apiClient.get<DNSLeakResult>(API_ROUTES.vpn.dnsLeakTest),
    onError: (error) => {
      toast.error('DNS leak test failed', { description: error.message });
    },
  });
}

export function useVerifyWireGuard() {
  return useMutation({
    mutationFn: () => apiClient.get<VPNVerifyResult>(API_ROUTES.vpn.wireguard.verify),
    onError: (error) => {
      toast.error('VPN verification failed', { description: error.message });
    },
  });
}

export function useSetKillSwitch() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (enabled: boolean) =>
      apiClient.put<{ status: string }>(API_ROUTES.vpn.killswitch, { enabled }),
    onSuccess: (_data, enabled) => {
      toast.success(`VPN kill switch ${enabled ? 'enabled' : 'disabled'}`);
      void queryClient.invalidateQueries({ queryKey: ['vpn', 'killswitch'] });
    },
    onError: (error) => {
      toast.error('Failed to update kill switch', { description: error.message });
    },
  });
}
