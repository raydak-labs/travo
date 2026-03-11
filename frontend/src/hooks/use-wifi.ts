import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type {
  WifiScanResult,
  WifiConnection,
  SavedNetwork,
  WifiMode,
  APConfig,
  MACConfig,
  GuestWifiConfig,
  RadioInfo,
  NetworkPriorityRequest,
} from '@shared/index';

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
    mutationFn: (params: { ssid: string; password: string; encryption?: string; hidden?: boolean }) =>
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

export function useWifiDelete() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (section: string) =>
      apiClient.del<{ status: string }>(`${API_ROUTES.wifi.deleteSaved}/${section}`),
    onSuccess: () => {
      toast.success('Network removed');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'saved'] });
    },
    onError: (error) => {
      toast.error('Failed to remove network', { description: error.message });
    },
  });
}

export function useSetNetworkPriority() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: NetworkPriorityRequest) =>
      apiClient.put<{ status: string }>(API_ROUTES.wifi.savedPriority, req),
    onSuccess: () => {
      toast.success('Network priority updated');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'saved'] });
    },
    onError: (error) => {
      toast.error('Failed to update priority', { description: error.message });
    },
  });
}

export function useAPConfigs() {
  return useQuery({
    queryKey: ['wifi', 'ap'],
    queryFn: () => apiClient.get<APConfig[]>(API_ROUTES.wifi.ap),
  });
}

export function useSetAPConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ section, config }: { section: string; config: APConfig }) =>
      apiClient.put<{ status: string }>(`${API_ROUTES.wifi.ap}/${section}`, config),
    onSuccess: () => {
      toast.success('AP configuration updated');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'ap'] });
    },
    onError: (error) => {
      toast.error('Failed to update AP config', { description: error.message });
    },
  });
}

export function useMACAddresses() {
  return useQuery({
    queryKey: ['wifi', 'mac'],
    queryFn: () => apiClient.get<MACConfig[]>(API_ROUTES.wifi.mac),
  });
}

export function useSetMAC() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (mac: string) => apiClient.put<{ status: string }>(API_ROUTES.wifi.mac, { mac }),
    onSuccess: () => {
      toast.success('MAC address updated');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'mac'] });
    },
    onError: (error) => {
      toast.error('Failed to update MAC address', { description: error.message });
    },
  });
}

export function useGuestWifi() {
  return useQuery({
    queryKey: ['wifi', 'guest'],
    queryFn: () => apiClient.get<GuestWifiConfig>(API_ROUTES.wifi.guest),
  });
}

export function useSetGuestWifi() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (cfg: GuestWifiConfig) =>
      apiClient.put<{ status: string }>(API_ROUTES.wifi.guest, cfg),
    onSuccess: () => {
      toast.success('Guest WiFi updated');
      void queryClient.invalidateQueries({ queryKey: ['wifi', 'guest'] });
    },
    onError: (error) => {
      toast.error('Failed to update guest WiFi', { description: error.message });
    },
  });
}

export function useRadioStatus() {
  return useQuery({
    queryKey: ['wifi', 'radio'],
    queryFn: () => apiClient.get<{ enabled: boolean }>(API_ROUTES.wifi.radio),
  });
}

export function useRadios() {
  return useQuery({
    queryKey: ['wifi', 'radios'],
    queryFn: () => apiClient.get<RadioInfo[]>(API_ROUTES.wifi.radios),
  });
}

export function useSetRadioEnabled() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (enabled: boolean) =>
      apiClient.put<{ status: string }>(API_ROUTES.wifi.radio, { enabled }),
    onSuccess: () => {
      toast.success('WiFi radio updated');
      void queryClient.invalidateQueries({ queryKey: ['wifi'] });
    },
    onError: (error) => {
      toast.error('Failed to update WiFi radio', { description: error.message });
    },
  });
}
