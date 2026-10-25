/**
 * The wireless network features that sit beside the connection: the repeater
 * options and radio reconcile, the MAC address and MAC policies, the guest
 * network, band switching, auto-reconnect and the WiFi schedule.
 */
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { finalizeWifiMutation } from '@/lib/wifi-apply';
import type {
  RepeaterOptions,
  GuestWifiConfig,
  BandSwitchConfig,
  WiFiSchedule,
  MACPolicies,
} from '@shared/index';
import { wifiMutationSettled } from './use-wifi-settled';
import {
  getAutoReconnect,
  getBandSwitching,
  getGuestWifi,
  getMAC,
  getMACPolicies,
  getRepeaterOptions,
  getWiFiSchedule,
  randomizeMAC,
  reconcileRepeaterAPLayout,
  setAutoReconnect,
  setBandSwitching,
  setGuestWifi,
  setMAC,
  setMACPolicies,
  setRepeaterOptions,
  setWiFiSchedule,
} from '@/api/endpoints';

export function useRepeaterOptions(enabled = true) {
  return useQuery({
    queryKey: ['wifi', 'repeater-options'],
    queryFn: () => getRepeaterOptions(),
    enabled,
  });
}

export function useSetRepeaterOptions() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: RepeaterOptions) => finalizeWifiMutation(setRepeaterOptions(body)),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'repeater-options'] });
    },
    onError: (error) => {
      toast.error('Failed to save repeater options', { description: error.message });
    },
  });
}

export function useRepeaterRadioReconcile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => finalizeWifiMutation(reconcileRepeaterAPLayout()),
    onSuccess: () => {
      toast.success('Repeater radio layout updated');
      wifiMutationSettled(queryClient, [
        ['wifi', 'health'],
        ['wifi', 'ap'],
        ['wifi', 'connection'],
      ]);
    },
    onError: (error) => {
      toast.error('Could not update radio layout', { description: error.message });
    },
  });
}

export function useMACAddresses() {
  return useQuery({
    queryKey: ['wifi', 'mac'],
    queryFn: () => getMAC(),
  });
}

export function useSetMAC() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (mac: string) => finalizeWifiMutation(setMAC({ mac })),
    onSuccess: () => {
      toast.success('MAC address updated');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'mac'] });
    },
    onError: (error) => {
      toast.error('Failed to update MAC address', { description: error.message });
    },
  });
}

export function useRandomizeMAC() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => finalizeWifiMutation(randomizeMAC()),
    onSuccess: (data) => {
      toast.success(`MAC randomized to ${data.mac}`);
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'mac'] });
    },
    onError: (error) => {
      toast.error('Failed to randomize MAC', { description: error.message });
    },
  });
}

export function useGuestWifi() {
  return useQuery({
    queryKey: ['wifi', 'guest'],
    queryFn: () => getGuestWifi(),
  });
}

export function useSetGuestWifi() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (cfg: GuestWifiConfig) => finalizeWifiMutation(setGuestWifi(cfg)),
    onSuccess: () => {
      toast.success('Guest WiFi updated');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'guest'] });
    },
    onError: (error) => {
      toast.error('Failed to update guest WiFi', { description: error.message });
    },
  });
}

export function useAutoReconnect() {
  return useQuery({
    queryKey: ['wifi', 'autoreconnect'],
    queryFn: () => getAutoReconnect(),
  });
}

export function useSetAutoReconnect() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (enabled: boolean) => setAutoReconnect({ enabled }),
    onSuccess: (_data, enabled) => {
      toast.success(enabled ? 'Auto-reconnect enabled' : 'Auto-reconnect disabled');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'autoreconnect'] });
    },
    onError: (error) => {
      toast.error('Failed to update auto-reconnect', { description: error.message });
    },
  });
}

export function useBandSwitching() {
  return useQuery({
    queryKey: ['wifi', 'band-switching'],
    queryFn: () => getBandSwitching(),
    refetchInterval: 10_000,
  });
}

export function useSetBandSwitching() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (config: BandSwitchConfig) => setBandSwitching(config),
    onSuccess: () => {
      // Without a success toast this toggle read from a 10s-polled query, so
      // enabling it looked like nothing happened.
      toast.success('Band switching updated');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'band-switching'] });
    },
    onError: (error) => {
      toast.error('Failed to update band switching config', { description: error.message });
    },
  });
}

export function useWiFiSchedule() {
  return useQuery({
    queryKey: ['wifi', 'schedule'],
    queryFn: () => getWiFiSchedule(),
  });
}

export function useSetWiFiSchedule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (schedule: WiFiSchedule) => setWiFiSchedule(schedule),
    onSuccess: () => {
      toast.success('WiFi schedule saved');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'schedule'] });
    },
    onError: (error) => {
      toast.error('Failed to save WiFi schedule', { description: error.message });
    },
  });
}

export function useMACPolicies() {
  return useQuery({
    queryKey: ['wifi', 'macPolicies'],
    queryFn: () => getMACPolicies(),
  });
}

export function useSetMACPolicies() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (policies: MACPolicies) => setMACPolicies(policies),
    onSuccess: () => {
      toast.success('MAC policies saved');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'macPolicies'] });
    },
    onError: (error) => {
      toast.error('Failed to save MAC policies', { description: error.message });
    },
  });
}
