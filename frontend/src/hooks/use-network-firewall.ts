/**
 * Firewall and the other advanced network tools: zones, port forwards,
 * diagnostics, wake-on-LAN, IPv6 and DNS-over-HTTPS.
 */
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type {
  AddPortForwardRequest,
  DiagnosticsRequest,
  DoHConfig,
  WoLRequest,
} from '@shared/index';
import {
  addPortForward,
  deletePortForward,
  getDoHConfig,
  getFirewallZones,
  getIPv6Status,
  getPortForwards,
  runDiagnostics,
  sendWoL,
  setDoHConfig,
  setIPv6Enabled,
} from '@/api/endpoints';

export function useFirewallZones() {
  return useQuery({
    queryKey: ['network', 'firewallZones'],
    queryFn: async () => {
      const res = await getFirewallZones();
      return res.zones;
    },
  });
}

export function usePortForwards() {
  return useQuery({
    queryKey: ['network', 'portForwards'],
    queryFn: async () => {
      const res = await getPortForwards();
      return res.rules;
    },
  });
}

export function useAddPortForward() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (rule: AddPortForwardRequest) => addPortForward(rule),
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
    mutationFn: (id: string) => deletePortForward(id),
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
    mutationFn: (req: DiagnosticsRequest) => runDiagnostics(req),
    onError: (error) => {
      toast.error('Diagnostic failed', { description: error.message });
    },
  });
}

export function useSendWoL() {
  return useMutation({
    mutationFn: (req: WoLRequest) => sendWoL(req),
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
    queryFn: ({ signal }) => getIPv6Status({ signal }),
  });
}

export function useSetIPv6Enabled() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (enabled: boolean) => setIPv6Enabled({ enabled }),
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
    queryFn: ({ signal }) => getDoHConfig({ signal }),
  });
}

export function useSetDoHConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (cfg: DoHConfig) => setDoHConfig(cfg),
    onSuccess: () => {
      toast.success('DNS-over-HTTPS settings saved');
      void queryClient.invalidateQueries({ queryKey: ['network', 'doh'] });
    },
    onError: (error) => {
      toast.error('Failed to save DoH settings', { description: error.message });
    },
  });
}
