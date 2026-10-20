/**
 * Device configuration hooks: hostname, timezone and NTP, the LED status and
 * its schedule, the hardware button actions, and the SSH key list.
 */
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { routeWithSegment } from '@/lib/api-url';
import { API_ROUTES } from '@shared/index';
import type {
  SetHostnameRequest,
  LEDStatus,
  SetLEDRequest,
  LEDSchedule,
  TimezoneConfig,
  NTPConfig,
  HardwareButton,
  ButtonActionsRequest,
  SSHKeysResponse,
  AddSSHKeyRequest,
} from '@shared/index';

export function useSetHostname() {
  return useMutation({
    mutationFn: (data: SetHostnameRequest) =>
      apiClient.put<{ status: string }>(API_ROUTES.system.hostname, data),
    onSuccess: () => {
      toast.success('Hostname updated');
    },
    onError: (error) => {
      toast.error('Failed to update hostname', { description: error.message });
    },
  });
}

export function useTimezone() {
  return useQuery({
    queryKey: ['system', 'timezone'],
    queryFn: () => apiClient.get<TimezoneConfig>(API_ROUTES.system.timezone),
  });
}

export function useSetTimezone() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (config: TimezoneConfig) =>
      apiClient.put<{ status: string }>(API_ROUTES.system.timezone, config),
    onSuccess: () => {
      toast.success('Timezone updated');
      void queryClient.invalidateQueries({ queryKey: ['system', 'timezone'] });
    },
    onError: (error) => {
      toast.error('Failed to update timezone', { description: error.message });
    },
  });
}

export function useNTPConfig() {
  return useQuery({
    queryKey: ['system', 'ntp'],
    queryFn: () => apiClient.get<NTPConfig>(API_ROUTES.system.ntp),
  });
}

export function useSyncNTP() {
  return useMutation({
    mutationFn: () => apiClient.post<{ status: string }>(API_ROUTES.system.ntpSync),
    onSuccess: () => {
      toast.success('NTP sync complete', {
        description: 'System clock synchronized with pool.ntp.org',
      });
    },
    onError: (error) => {
      toast.error('NTP sync failed', { description: error.message });
    },
  });
}

export function useSetNTPConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (config: { enabled: boolean; servers: string[] }) =>
      apiClient.put<{ status: string }>(API_ROUTES.system.ntp, config),
    onSuccess: () => {
      toast.success('NTP configuration updated');
      void queryClient.invalidateQueries({ queryKey: ['system', 'ntp'] });
    },
    onError: (error) => {
      toast.error('Failed to update NTP configuration', { description: error.message });
    },
  });
}

export function useLEDStatus() {
  return useQuery({
    queryKey: ['system', 'leds'],
    queryFn: () => apiClient.get<LEDStatus>(API_ROUTES.system.leds),
  });
}

export function useSetLEDStealth() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: SetLEDRequest) => apiClient.put<LEDStatus>(API_ROUTES.system.leds, data),
    onSuccess: () => {
      toast.success('LED mode updated');
      void queryClient.invalidateQueries({ queryKey: ['system', 'leds'] });
    },
    onError: (error) => {
      toast.error('Failed to update LED mode', { description: error.message });
    },
  });
}

export function useLEDSchedule() {
  return useQuery({
    queryKey: ['system', 'leds', 'schedule'],
    queryFn: () => apiClient.get<LEDSchedule>(API_ROUTES.system.ledsSchedule),
  });
}

export function useSetLEDSchedule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: LEDSchedule) =>
      apiClient.put<LEDSchedule>(API_ROUTES.system.ledsSchedule, data),
    onSuccess: () => {
      toast.success('LED schedule updated');
      void queryClient.invalidateQueries({ queryKey: ['system', 'leds', 'schedule'] });
    },
    onError: (error) => {
      toast.error('Failed to update LED schedule', { description: error.message });
    },
  });
}

export function useHardwareButtons() {
  return useQuery({
    queryKey: ['system', 'buttons'],
    queryFn: () => apiClient.get<HardwareButton[]>(API_ROUTES.system.buttons),
  });
}

export function useSetButtonActions() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: ButtonActionsRequest) =>
      apiClient.put<{ status: string }>(API_ROUTES.system.buttonActions, req),
    onSuccess: () => {
      toast.success('Button actions saved');
      void queryClient.invalidateQueries({ queryKey: ['system', 'buttons'] });
    },
    onError: (error) => {
      toast.error('Failed to save button actions', { description: error.message });
    },
  });
}

export function useSSHKeys() {
  return useQuery({
    queryKey: ['system', 'ssh-keys'],
    queryFn: async () => {
      const res = await apiClient.get<SSHKeysResponse>(API_ROUTES.system.sshKeys);
      return res.keys;
    },
  });
}

export function useAddSSHKey() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: AddSSHKeyRequest) =>
      apiClient.post<{ ok: boolean }>(API_ROUTES.system.sshKeys, req),
    onSuccess: () => {
      toast.success('SSH key added');
      void queryClient.invalidateQueries({ queryKey: ['system', 'ssh-keys'] });
    },
    onError: (error) => {
      toast.error('Failed to add SSH key', { description: error.message });
    },
  });
}

export function useDeleteSSHKey() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (index: number) =>
      apiClient.del<{ ok: boolean }>(routeWithSegment(API_ROUTES.system.sshKeys, index)),
    onSuccess: () => {
      toast.success('SSH key deleted');
      void queryClient.invalidateQueries({ queryKey: ['system', 'ssh-keys'] });
    },
    onError: (error) => {
      toast.error('Failed to delete SSH key', { description: error.message });
    },
  });
}
