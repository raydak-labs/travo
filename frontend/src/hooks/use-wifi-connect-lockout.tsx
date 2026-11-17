import { WifiLockoutDialog } from '@/components/wifi/wifi-lockout-dialog';
import { useWifiLockout } from '@/hooks/use-wifi-lockout';
import { useWifiConnect, type WifiConnectParams } from './use-wifi-connection';

type ConnectOptions = { onSuccess?: () => void };

/** Connect plus the server-refused, explicit self-disconnect acknowledgement. */
export function useWifiConnectLockout() {
  const mutation = useWifiConnect();
  const lockout = useWifiLockout();

  function connect(params: WifiConnectParams, options: ConnectOptions = {}) {
    mutation.mutate(params, {
      onSuccess: options.onSuccess,
      onError: (error) => {
        lockout.onLockout(error, () => {
          mutation.mutate({ ...params, acknowledge_lockout: true }, options);
        });
      },
    });
  }

  const lockoutDialog = (
    <WifiLockoutDialog
      open={lockout.open}
      isPending={mutation.isPending}
      onCancel={lockout.dismiss}
      onConfirm={lockout.acknowledge}
    />
  );

  return { mutation, connect, lockoutDialog };
}
