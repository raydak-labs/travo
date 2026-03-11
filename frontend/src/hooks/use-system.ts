import { useQuery, useMutation } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type { SystemInfo, SystemStats, LogResponse, ChangePasswordRequest, SetHostnameRequest, LEDStatus, SetLEDRequest } from '@shared/index';

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
