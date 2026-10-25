import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { refreshRouterState } from '@/lib/router-state-refresh';
import type { SplitTunnelConfig } from '@shared/index';
import {
  activateWireguardProfile,
  addWireguardProfile,
  dNSLeakTest,
  deleteWireguardProfile,
  getKillSwitch,
  getSplitTunnel,
  getTailscale,
  getTailscaleSSH,
  getVpnStatus,
  getWireguard,
  getWireguardProfiles,
  getWireguardStatus,
  runWireGuardSpeedTest,
  setKillSwitch,
  setSplitTunnel,
  setTailscaleExitNode,
  setTailscaleSSH,
  tailscaleAuth,
  toggleTailscale,
  toggleWireguard,
  verifyWireguard,
} from '@/api/endpoints';

export function useVpnStatus() {
  return useQuery({
    queryKey: ['vpn', 'status'],
    queryFn: () => getVpnStatus(),
  });
}

export function useWireguardConfig() {
  return useQuery({
    queryKey: ['vpn', 'wireguard'],
    queryFn: () => getWireguard(),
  });
}

export function useWireguardStatus() {
  return useQuery({
    queryKey: ['vpn', 'wireguard', 'status'],
    queryFn: () => getWireguardStatus(),
    refetchInterval: 10000, // refresh every 10 seconds for live stats
  });
}

export function useToggleWireguard() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (enable: boolean) => toggleWireguard({ enabled: enable }),
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
    queryFn: () => getTailscale(),
  });
}

export function useToggleTailscale() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (enable: boolean) => toggleTailscale({ enabled: enable }),
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
    queryFn: () => getWireguardProfiles(),
  });
}

export function useAddWireguardProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: { name: string; config: string }) => addWireguardProfile(data),
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
    mutationFn: (id: string) => deleteWireguardProfile(id),
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
    mutationFn: (id: string) => activateWireguardProfile(id),
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
    queryFn: () => getKillSwitch(),
  });
}

export function useDNSLeakTest() {
  return useMutation({
    mutationFn: () => dNSLeakTest(),
    onError: (error) => {
      toast.error('DNS leak test failed', { description: error.message });
    },
  });
}

/** Speed test with download + ping bound to WireGuard (wg0); requires tunnel enabled and up. */
export function useRunWireGuardSpeedTest() {
  return useMutation({
    mutationFn: () => runWireGuardSpeedTest(),
    onError: (error) => {
      toast.error('VPN speed test failed', { description: error.message });
    },
  });
}

export function useVerifyWireGuard() {
  return useMutation({
    mutationFn: () => verifyWireguard(),
    onError: (error) => {
      toast.error('VPN verification failed', { description: error.message });
    },
  });
}

export function useSetKillSwitch() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (enabled: boolean) => setKillSwitch({ enabled }),
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
      tailscaleAuth({
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
    mutationFn: (nodeIP: string) => setTailscaleExitNode({ node_ip: nodeIP }),
    onSuccess: () => {
      toast.success('Exit node updated');
      void queryClient.invalidateQueries({ queryKey: ['vpn'] });
      void refreshRouterState(queryClient, [
        ['vpn', 'status'],
        ['vpn', 'wireguard', 'status'],
        ['vpn', 'tailscale'],
        ['network', 'status'],
      ]);
    },
    onError: (error) => {
      toast.error('Failed to set exit node', { description: error.message });
    },
  });
}

export function useSplitTunnel() {
  return useQuery({
    queryKey: ['vpn', 'splitTunnel'],
    queryFn: () => getSplitTunnel(),
  });
}

export function useSetSplitTunnel() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (cfg: SplitTunnelConfig) => setSplitTunnel(cfg),
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
    queryFn: () => getTailscaleSSH(),
  });
}

export function useSetTailscaleSSH() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (enabled: boolean) => setTailscaleSSH({ enabled }),
    onSuccess: () => {
      toast.success('Tailscale SSH setting updated');
      void queryClient.invalidateQueries({ queryKey: ['vpn', 'tailscale', 'ssh'] });
    },
    onError: (error) => {
      toast.error('Failed to update Tailscale SSH', { description: error.message });
    },
  });
}
