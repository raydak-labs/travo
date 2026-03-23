import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type {
  SystemInfo,
  SystemStats,
  LogResponse,
  ChangePasswordRequest,
  SetHostnameRequest,
  LEDStatus,
  SetLEDRequest,
  LEDSchedule,
  TimezoneConfig,
  NTPConfig,
  SetupStatus,
} from '@shared/index';

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

export function useShutdown() {
  return useMutation({
    mutationFn: () => apiClient.post<{ status: string }>(API_ROUTES.system.shutdown),
    onSuccess: () => {
      toast.success('Shutdown initiated', {
        description: 'The device is powering off. You will need physical access to turn it back on.',
      });
    },
    onError: (error) => {
      toast.error('Failed to shut down', { description: error.message });
    },
  });
}

export function useFactoryReset() {
  return useMutation({
    mutationFn: () => apiClient.post<{ status: string }>(API_ROUTES.system.factoryReset),
    onSuccess: () => {
      toast.success('Factory reset initiated', {
        description: 'Device will restart with default settings.',
      });
    },
    onError: (error) => {
      toast.error('Factory reset failed', { description: error.message });
    },
  });
}

export function useSystemLogs(service?: string, level?: string) {
  return useQuery({
    queryKey: ['system', 'logs', service ?? '', level ?? ''],
    queryFn: () => {
      const params = new URLSearchParams();
      if (service) params.set('service', service);
      if (level) params.set('level', level);
      const qs = params.toString();
      const url = qs ? `${API_ROUTES.system.logs}?${qs}` : API_ROUTES.system.logs;
      return apiClient.get<LogResponse>(url);
    },
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

export function useFirmwareUpgrade() {
  return useMutation({
    mutationFn: async ({ file, keepSettings }: { file: File; keepSettings: boolean }) => {
      const formData = new FormData();
      formData.append('firmware', file);
      formData.append('keep_settings', String(keepSettings));
      const response = await fetch(API_ROUTES.system.firmwareUpgrade, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${localStorage.getItem('openwrt-auth-token') || sessionStorage.getItem('openwrt-auth-token') || ''}`,
        },
        body: formData,
      });
      if (!response.ok) {
        const data = await response.json();
        throw new Error(data.error || 'Firmware upgrade failed');
      }
      return response.json();
    },
    onSuccess: () => {
      toast.success('Firmware upgrade initiated', {
        description: 'Device will reboot with the new firmware.',
      });
    },
    onError: (error) => {
      toast.error('Firmware upgrade failed', { description: error.message });
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
      toast.success('NTP sync complete', { description: 'System clock synchronized with pool.ntp.org' });
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

export function useSetupStatus() {
  return useQuery({
    queryKey: ['system', 'setup-complete'],
    queryFn: () => apiClient.get<SetupStatus>(API_ROUTES.system.setupComplete),
  });
}

export function useCompleteSetup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => apiClient.post<{ status: string }>(API_ROUTES.system.setupComplete),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['system', 'setup-complete'] });
    },
  });
}
