/**
 * Device configuration hooks: hostname, timezone and NTP, the LED status and
 * its schedule, the hardware button actions, and the SSH key list.
 */
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import type {
  AddSSHKeyRequest,
  ButtonActionsRequest,
  LEDSchedule,
  SetHostnameRequest,
  SetLEDRequest,
  TimezoneConfig,
} from '@shared/index';
import {
  addSSHKey,
  deleteSSHKey,
  getButtons,
  getLEDSchedule,
  getLEDStatus,
  getNTPConfig,
  getSSHKeys,
  getTimezone,
  setButtonActions,
  setHostname,
  setLEDSchedule,
  setLEDStealth,
  setNTPConfig,
  setTimezone,
  syncNTP,
} from '@/api/endpoints';

export function useSetHostname() {
  return useMutation({
    mutationFn: (data: SetHostnameRequest) => setHostname(data),
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
    queryFn: () => getTimezone(),
  });
}

export function useSetTimezone() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (config: TimezoneConfig) => setTimezone(config),
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
    queryFn: () => getNTPConfig(),
  });
}

export function useSyncNTP() {
  return useMutation({
    mutationFn: () => syncNTP(),
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
    mutationFn: (config: { enabled: boolean; servers: string[] }) => setNTPConfig(config),
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
    queryFn: () => getLEDStatus(),
  });
}

export function useSetLEDStealth() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: SetLEDRequest) => setLEDStealth(data),
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
    queryFn: () => getLEDSchedule(),
  });
}

export function useSetLEDSchedule() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: LEDSchedule) => setLEDSchedule(data),
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
    queryFn: () => getButtons(),
  });
}

export function useSetButtonActions() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: ButtonActionsRequest) => setButtonActions(req),
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
      const res = await getSSHKeys();
      return res.keys;
    },
  });
}

export function useAddSSHKey() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: AddSSHKeyRequest) => addSSHKey(req),
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
    mutationFn: (index: number) => deleteSSHKey(String(index)),
    onSuccess: () => {
      toast.success('SSH key deleted');
      void queryClient.invalidateQueries({ queryKey: ['system', 'ssh-keys'] });
    },
    onError: (error) => {
      toast.error('Failed to delete SSH key', { description: error.message });
    },
  });
}
