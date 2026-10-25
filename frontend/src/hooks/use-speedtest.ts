import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { isSpeedtestServiceStatus, isSpeedTestResult } from '@shared/index';
import type { SpeedTestResult } from '@shared/index';
import {
  getSpeedtestServiceStatus,
  installSpeedtestCLI,
  runSpeedtestCLI,
  uninstallSpeedtestCLI,
} from '@/api/endpoints';

export function useSpeedtestServiceStatus() {
  return useQuery({
    queryKey: ['speedtest-service'],
    queryFn: async () => {
      const data = await getSpeedtestServiceStatus();
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
    mutationFn: () => installSpeedtestCLI(),
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
    mutationFn: () => uninstallSpeedtestCLI(),
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
      const data = await runSpeedtestCLI();
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
