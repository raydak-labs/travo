import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { refreshRouterState } from '@/lib/router-state-refresh';
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
  SplitTunnelConfig,
  TailscaleSSHStatus,
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
      void refreshRouterState(queryClient, [
        ['vpn', 'status'],
        ['vpn', 'wireguard', 'status'],
        ['network', 'status'],
        ['network', 'dns'],
      ]);
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
      void refreshRouterState(queryClient, [
        ['vpn', 'status'],
        ['vpn', 'tailscale'],
        ['network', 'status'],
      ]);
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
      void refreshRouterState(queryClient, [
        ['vpn', 'status'],
        ['vpn', 'wireguard', 'status'],
        ['vpn', 'wireguard', 'profiles'],
      ]);
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

export function useTailscaleAuth() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (authKey?: string) =>
      apiClient.post<{ auth_url?: string; status: string }>(API_ROUTES.vpn.tailscale.auth, {
        auth_key: authKey ?? '',
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['vpn', 'tailscale'] });
    },
    onError: (error) => {
      toast.error('Tailscale auth failed', { description: error.message });
    },
  });
}

export function useSetTailscaleExitNode() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (nodeIP: string) =>
      apiClient.post<{ status: string }>(API_ROUTES.vpn.tailscale.exitNode, { exit_node: nodeIP }),
    onSuccess: () => {
      toast.success('Exit node updated');
      void queryClient.invalidateQueries({ queryKey: ['vpn', 'tailscale'] });
    },
    onError: (error) => {
      toast.error('Failed to set exit node', { description: error.message });
    },
  });
}

export function useSplitTunnel() {
  return useQuery({
    queryKey: ['vpn', 'splitTunnel'],
    queryFn: () => apiClient.get<SplitTunnelConfig>(API_ROUTES.vpn.splitTunnel),
  });
}

export function useSetSplitTunnel() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (cfg: SplitTunnelConfig) =>
      apiClient.put<{ status: string }>(API_ROUTES.vpn.splitTunnel, cfg),
    onSuccess: () => {
      toast.success('Split tunnel config saved');
      void queryClient.invalidateQueries({ queryKey: ['vpn', 'splitTunnel'] });
    },
    onError: (error) => {
      toast.error('Failed to save split tunnel config', { description: error.message });
    },
  });
}

export function useTailscaleSSH() {
  return useQuery({
    queryKey: ['vpn', 'tailscale', 'ssh'],
    queryFn: () => apiClient.get<TailscaleSSHStatus>(API_ROUTES.vpn.tailscale.ssh),
  });
}

export function useSetTailscaleSSH() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (enabled: boolean) =>
      apiClient.put<{ ok: boolean }>(API_ROUTES.vpn.tailscale.ssh, { enabled }),
    onSuccess: () => {
      toast.success('Tailscale SSH setting updated');
      void queryClient.invalidateQueries({ queryKey: ['vpn', 'tailscale', 'ssh'] });
    },
    onError: (error) => {
      toast.error('Failed to update Tailscale SSH', { description: error.message });
    },
  });
}
