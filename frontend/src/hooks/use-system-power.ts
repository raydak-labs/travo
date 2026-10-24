/**
 * Device lifecycle operations: reboot, shutdown, factory reset, firmware
 * upgrade, and the configuration backup/restore pair.
 */
import { useMutation } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';

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
        description:
          'The device is powering off. You will need physical access to turn it back on.',
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

export function useFirmwareUpgrade() {
  return useMutation({
    mutationFn: async ({ file, keepSettings }: { file: File; keepSettings: boolean }) => {
      const formData = new FormData();
      formData.append('firmware', file);
      formData.append('keep_settings', String(keepSettings));
      return apiClient.postForm<{ status?: string }>(API_ROUTES.system.firmwareUpgrade, formData);
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

export function useBackup() {
  return useMutation({
    mutationFn: async () => {
      // Routed through the shared client so an expired session triggers the
      // global 401 handling instead of a bare "Backup failed".
      const blob = await apiClient.getBlob(API_ROUTES.system.backup);
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
      return apiClient.postForm<{ status?: string }>(API_ROUTES.system.restore, formData);
    },
    onSuccess: () => {
      toast.success('Configuration restored. Reboot to apply changes.');
    },
    onError: (error) => {
      toast.error('Failed to restore configuration', { description: error.message });
    },
  });
}
