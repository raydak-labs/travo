import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { configureUSBTether, getUSBTetherStatus, unconfigureUSBTether } from '@/api/endpoints';

export function useUSBTetherStatus() {
  return useQuery({
    queryKey: ['usb-tethering'],
    queryFn: () => getUSBTetherStatus(),
    refetchInterval: 10_000,
  });
}

export function useConfigureUSBTether() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (ifaceName: string) =>
      configureUSBTether({
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
    mutationFn: () => unconfigureUSBTether(),
    onSuccess: () => {
      toast.success('USB tethering removed');
      void queryClient.invalidateQueries({ queryKey: ['usb-tethering'] });
    },
    onError: (error: Error) => {
      toast.error('Failed to remove USB tethering', { description: error.message });
    },
  });
}
