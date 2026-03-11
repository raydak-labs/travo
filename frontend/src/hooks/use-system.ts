import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type { SystemInfo, SystemStats, LogResponse, ChangePasswordRequest, SetHostnameRequest, LEDStatus, SetLEDRequest, TimezoneConfig } from '@shared/index';

export function useSystemInfo() {
  return useQuery({
    queryKey: ['system', 'info'],
    queryFn: () => apiClient.get<SystemInfo>(API_ROUTES.system.info),
  });
}

export function useSystemStats() {
  return useQuery({
    queryKey: ['system', 'stats'],
    queryFn: () => apiClient.get<SystemStats>(API_ROUTES.system.stats),
    refetchInterval: 5_000,
  });
}

export function useReboot() {
  return useMutation({
    mutationFn: () => apiClient.post<{ status: string }>(API_ROUTES.system.reboot),
    onSuccess: () => {
      toast.success('Reboot initiated', { description: 'The device is restarting…' });
    },
    onError: (error) => {
      toast.error('Failed to reboot', { description: error.message });
    },
  });
}

export function useSystemLogs() {
  return useQuery({
    queryKey: ['system', 'logs'],
    queryFn: () => apiClient.get<LogResponse>(API_ROUTES.system.logs),
  });
}

export function useKernelLogs() {
  return useQuery({
    queryKey: ['system', 'logs', 'kernel'],
    queryFn: () => apiClient.get<LogResponse>(API_ROUTES.system.kernelLogs),
  });
}

export function useChangePassword() {
  return useMutation({
    mutationFn: (data: ChangePasswordRequest) =>
      apiClient.put<{ status: string }>(API_ROUTES.auth.password, data),
    onSuccess: () => {
      toast.success('Password changed successfully');
    },
    onError: (error) => {
      toast.error('Failed to change password', { description: error.message });
    },
  });
}

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

export function useLEDStatus() {
  return useQuery({
    queryKey: ['system', 'leds'],
    queryFn: () => apiClient.get<LEDStatus>(API_ROUTES.system.leds),
  });
}

export function useSetLEDStealth() {
  return useMutation({
    mutationFn: (data: SetLEDRequest) =>
      apiClient.put<LEDStatus>(API_ROUTES.system.leds, data),
    onSuccess: () => {
      toast.success('LED mode updated');
    },
    onError: (error) => {
      toast.error('Failed to update LED mode', { description: error.message });
    },
  });
}

export function useTimezone() {
  return useQuery({
    queryKey: ['system', 'timezone'],
    queryFn: () => apiClient.get<TimezoneConfig>(API_ROUTES.system.timezone),
  });
}

export function useBackup() {
  return useMutation({
    mutationFn: async () => {
      const response = await fetch(API_ROUTES.system.backup, {
        headers: {
          Authorization: `Bearer ${localStorage.getItem('token') || sessionStorage.getItem('token') || ''}`,
        },
      });
      if (!response.ok) throw new Error('Backup failed');
      const blob = await response.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'openwrt-backup.tar.gz';
      a.click();
      URL.revokeObjectURL(url);
    },
    onSuccess: () => {
      toast.success('Backup downloaded');
    },
    onError: (error) => {
      toast.error('Failed to create backup', { description: error.message });
    },
  });
}

export function useRestore() {
  return useMutation({
    mutationFn: async (file: File) => {
      const formData = new FormData();
      formData.append('backup', file);
      const response = await fetch(API_ROUTES.system.restore, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${localStorage.getItem('token') || sessionStorage.getItem('token') || ''}`,
        },
        body: formData,
      });
      if (!response.ok) {
        const data = await response.json();
        throw new Error(data.error || 'Restore failed');
      }
      return response.json();
    },
    onSuccess: () => {
      toast.success('Configuration restored. Reboot to apply changes.');
    },
    onError: (error) => {
      toast.error('Failed to restore configuration', { description: error.message });
    },
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
