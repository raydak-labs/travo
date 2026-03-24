import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { apiClient } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type { USBTetherStatus } from '@shared/index';

export function useUSBTetherStatus() {
  return useQuery({
    queryKey: ['usb-tethering'],
    queryFn: () => apiClient.get<USBTetherStatus>(API_ROUTES.network.usbTethering),
    refetchInterval: 10_000,
  });
}

export function useConfigureUSBTether() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (ifaceName: string) =>
      apiClient.post<{ status: string }>(API_ROUTES.network.usbTetheringConfigure, {
        interface: ifaceName,
      }),
    onSuccess: () => {
      toast.success('USB tethering configured as WAN source');
      void queryClient.invalidateQueries({ queryKey: ['usb-tethering'] });
    },
    onError: (error: Error) => {
      toast.error('Failed to configure USB tethering', { description: error.message });
    },
  });
}

export function useUnconfigureUSBTether() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      apiClient.post<{ status: string }>(API_ROUTES.network.usbTetheringUnconfigure, {}),
    onSuccess: () => {
      toast.success('USB tethering removed');
      void queryClient.invalidateQueries({ queryKey: ['usb-tethering'] });
    },
    onError: (error: Error) => {
      toast.error('Failed to remove USB tethering', { description: error.message });
    },
  });
}
