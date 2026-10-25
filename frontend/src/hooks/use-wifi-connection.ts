/**
 * The client-side wireless surface: the scan, the current connection, the
 * health probe, and the mutations that join, leave, or re-prioritize a
 * saved network.
 */
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { finalizeWifiMutation } from '@/lib/wifi-apply';
import { refreshRouterState } from '@/lib/router-state-refresh';
import type { WifiMode, NetworkPriorityRequest } from '@shared/index';
import { wifiMutationSettled } from './use-wifi-settled';
import {
  deleteSavedNetwork,
  getSavedNetworks,
  getWiFiConnection,
  getWiFiHealth,
  setNetworkPriority,
  setWiFiMode,
  wiFiConnect,
  wiFiDisconnect,
  wiFiScan,
} from '@/api/endpoints';

export function useWifiScan(enabled = true) {
  return useQuery({
    queryKey: ['wifi', 'scan'],
    queryFn: () => wiFiScan(),
    enabled,
  });
}

export function useWifiConnection() {
  return useQuery({
    queryKey: ['wifi', 'connection'],
    queryFn: () => getWiFiConnection(),
  });
}

export function useWifiHealth() {
  return useQuery({
    queryKey: ['wifi', 'health'],
    queryFn: () => getWiFiHealth(),
    refetchInterval: 15_000,
  });
}

export function useWifiConnect() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: {
      ssid: string;
      password: string;
      encryption?: string;
      hidden?: boolean;
      /** Preferred band for dual-band networks (2.4ghz, 5ghz, 6ghz) */
      band?: string;
    }) => finalizeWifiMutation(wiFiConnect(params)),
    onSuccess: (_data, variables) => {
      toast.success(`Connected to ${variables.ssid}`);
      wifiMutationSettled(queryClient, [
        ['wifi', 'connection'],
        ['wifi', 'saved'],
        ['network', 'status'],
      ]);
    },
    onError: (error) => {
      toast.error('WiFi connection failed', { description: error.message });
    },
  });
}

export function useWifiDisconnect() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => finalizeWifiMutation(wiFiDisconnect()),
    onSuccess: () => {
      toast.success('Disconnected from WiFi');
      wifiMutationSettled(queryClient, [
        ['wifi', 'connection'],
        ['wifi', 'saved'],
        ['network', 'status'],
      ]);
    },
    onError: (error) => {
      toast.error('Failed to disconnect', { description: error.message });
    },
  });
}

export function useWifiMode() {
  const queryClient = useQueryClient();
  return useMutation({
    // acknowledge_lockout rides along with the mode so the caller can re-send
    // the SAME request once the operator has ticked the lockout dialog. The
    // hook must not decide that: the dialog does, in the page.
    mutationFn: ({
      mode,
      acknowledge_lockout,
    }: {
      mode: WifiMode;
      acknowledge_lockout?: boolean;
    }) =>
      finalizeWifiMutation(
        setWiFiMode({
          mode,
          ...(acknowledge_lockout ? { acknowledge_lockout: true } : {}),
        }),
      ),
    onSuccess: (_data, { mode }) => {
      toast.success(`WiFi mode changed to ${mode}`);
      wifiMutationSettled(queryClient, [
        ['wifi', 'connection'],
        ['wifi', 'saved'],
        ['wifi', 'ap'],
        ['network', 'status'],
      ]);
    },
    onError: (error) => {
      // The lockout refusal also raises its own acknowledgement dialog, but it
      // is NOT swallowed here: this hook is shared by pages that do not wire
      // that dialog, and silence there would leave a refused change with no
      // explanation at all.
      toast.error('Failed to change WiFi mode', { description: error.message });
    },
  });
}

export function useSavedNetworks() {
  return useQuery({
    queryKey: ['wifi', 'saved'],
    queryFn: () => getSavedNetworks(),
  });
}

export function useWifiDelete() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (section: string) => finalizeWifiMutation(deleteSavedNetwork(section)),
    onSuccess: () => {
      toast.success('Network removed');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'saved'] });
      void refreshRouterState(queryClient, [['wifi', 'saved']]);
    },
    onError: (error) => {
      toast.error('Failed to remove network', { description: error.message });
    },
  });
}

export function useSetNetworkPriority() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: NetworkPriorityRequest) => setNetworkPriority(req),
    onSuccess: () => {
      toast.success('Network priority updated');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'saved'] });
      void refreshRouterState(queryClient, [['wifi', 'saved']]);
    },
    onError: (error) => {
      toast.error('Failed to update priority', { description: error.message });
    },
  });
}
