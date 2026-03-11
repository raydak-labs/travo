import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type { WifiScanResult, WifiConnection, SavedNetwork, WifiMode } from '@shared/index';

export function useWifiScan(enabled = true) {
  return useQuery({
    queryKey: ['wifi', 'scan'],
    queryFn: () => apiClient.get<WifiScanResult[]>(API_ROUTES.wifi.scan),
    enabled,
  });
}

export function useWifiConnection() {
  return useQuery({
    queryKey: ['wifi', 'connection'],
    queryFn: () => apiClient.get<WifiConnection>(API_ROUTES.wifi.connection),
  });
}

export function useWifiConnect() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: { ssid: string; password: string; encryption?: string }) =>
      apiClient.post<{ status: string }>(API_ROUTES.wifi.connect, params),
    onSuccess: (_data, variables) => {
      toast.success(`Connected to ${variables.ssid}`);
      // Delay refetch so the wireless subsystem has time to apply changes after wifi reload
      setTimeout(() => {
        void queryClient.invalidateQueries({ queryKey: ['wifi'] });
      }, 3000);
    },
    onError: (error) => {
      toast.error('WiFi connection failed', { description: error.message });
    },
  });
}

export function useWifiDisconnect() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => apiClient.post<{ status: string }>(API_ROUTES.wifi.disconnect),
    onSuccess: () => {
      toast.success('Disconnected from WiFi');
      // Delay refetch so the wireless subsystem has time to apply changes after wifi reload
      setTimeout(() => {
        void queryClient.invalidateQueries({ queryKey: ['wifi'] });
      }, 2000);
    },
    onError: (error) => {
      toast.error('Failed to disconnect', { description: error.message });
    },
  });
}

export function useWifiMode() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (mode: WifiMode) =>
      apiClient.put<{ success: boolean; mode: string }>(API_ROUTES.wifi.mode, { mode }),
    onSuccess: (_data, mode) => {
      toast.success(`WiFi mode changed to ${mode}`);
      void queryClient.invalidateQueries({ queryKey: ['wifi'] });
    },
    onError: (error) => {
      toast.error('Failed to change WiFi mode', { description: error.message });
    },
  });
}

export function useSavedNetworks() {
  return useQuery({
    queryKey: ['wifi', 'saved'],
    queryFn: () => apiClient.get<SavedNetwork[]>(API_ROUTES.wifi.saved),
  });
}
