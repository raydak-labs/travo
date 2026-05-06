import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES, isSpeedtestServiceStatus, isSpeedTestResult } from '@shared/index';
import type { SpeedTestResult } from '@shared/index';

export function useSpeedtestServiceStatus() {
  return useQuery({
    queryKey: ['speedtest-service'],
    queryFn: async () => {
      const data = await apiClient.get<unknown>(API_ROUTES.system.speedtestService);
      if (!isSpeedtestServiceStatus(data)) {
        throw new Error('Invalid speedtest service status response');
      }
      return data;
    },
  });
}

export function useInstallSpeedtestCLI() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => apiClient.post<{ ok: boolean }>(API_ROUTES.system.speedtestServiceInstall),
    onSuccess: () => {
      toast.success('speedtest CLI installed');
      void queryClient.invalidateQueries({ queryKey: ['speedtest-service'] });
    },
    onError: (error) => {
      toast.error('Failed to install speedtest CLI', { description: error.message });
    },
  });
}

export function useUninstallSpeedtestCLI() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => apiClient.post<{ ok: boolean }>(API_ROUTES.system.speedtestServiceUninstall),
    onSuccess: () => {
      toast.success('speedtest CLI uninstalled');
      void queryClient.invalidateQueries({ queryKey: ['speedtest-service'] });
    },
    onError: (error) => {
      toast.error('Failed to uninstall speedtest CLI', { description: error.message });
    },
  });
}

export function useRunSpeedtest() {
  const queryClient = useQueryClient();
  return useMutation<SpeedTestResult, Error>({
    mutationFn: async () => {
      const data = await apiClient.post<unknown>(API_ROUTES.system.speedtestServiceRun);
      if (!isSpeedTestResult(data)) {
        throw new Error('Invalid speedtest result response');
      }
      return data;
    },
    onSuccess: () => {
      toast.success('Speed test completed');
      void queryClient.invalidateQueries({ queryKey: ['speedtest-service'] });
    },
    onError: (error) => {
      toast.error('Speed test failed', { description: error.message });
    },
  });
}