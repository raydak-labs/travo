import { useState } from 'react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import {
  useWifiDisconnect,
  useWifiConnect,
  useWifiConnection,
  useRadioStatus,
  useSetRadioEnabled,
} from '@/hooks/use-wifi';
import { useToggleWireguard, useVpnStatus } from '@/hooks/use-vpn';
import { useReboot } from '@/hooks/use-system';
import { QuickActionsButtonRow } from '@/pages/dashboard/quick-actions-button-row';

export function QuickActions() {
  const [showRebootDialog, setShowRebootDialog] = useState(false);

  const { data: wifiConnection } = useWifiConnection();
  const { data: vpnStatusList } = useVpnStatus();
  const { data: radioStatus } = useRadioStatus();
  const wifiDisconnect = useWifiDisconnect();
  const wifiReconnect = useWifiConnect();
  const toggleWireguard = useToggleWireguard();
  const setRadioEnabled = useSetRadioEnabled();
  const reboot = useReboot();

  const radioEnabled = radioStatus?.enabled ?? true;
  const wg = vpnStatusList?.find((s) => s.type === 'wireguard');
  const vpnEnabled = wg?.enabled ?? false;
  const desiredVpnEnabled = toggleWireguard.isPending ? toggleWireguard.variables : undefined;

  const handleRestartWifi = () => {
    if (wifiDisconnect.isPending || wifiReconnect.isPending) return;
    wifiDisconnect.mutate(undefined, {
      onSuccess: () => {
        if (wifiConnection?.ssid) {
          wifiReconnect.mutate({ ssid: wifiConnection.ssid, password: '' });
        }
      },
    });
  };

  const handleToggleVpn = () => {
    toggleWireguard.mutate(!vpnEnabled);
  };

  const handleToggleRadio = () => {
    setRadioEnabled.mutate(!radioEnabled);
  };

  const handleReboot = () => {
    reboot.mutate(undefined, {
      onSuccess: () => setShowRebootDialog(false),
    });
  };

  const wifiRestarting = wifiDisconnect.isPending || wifiReconnect.isPending;

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">Quick Actions</CardTitle>
        </CardHeader>
        <CardContent>
          <QuickActionsButtonRow
            radioEnabled={radioEnabled}
            setRadioPending={setRadioEnabled.isPending}
            onToggleRadio={handleToggleRadio}
            wifiRestarting={wifiRestarting}
            onRestartWifi={handleRestartWifi}
            vpnTogglePending={toggleWireguard.isPending}
            desiredVpnEnabled={desiredVpnEnabled}
            vpnEnabled={vpnEnabled}
            onToggleVpn={handleToggleVpn}
            rebootPending={reboot.isPending}
            onOpenRebootDialog={() => setShowRebootDialog(true)}
          />
        </CardContent>
      </Card>

      <ConfirmDialog
        open={showRebootDialog}
        onOpenChange={setShowRebootDialog}
        title="Reboot System"
        description="The router will reboot and be temporarily unreachable."
        warningText="You will lose your connection for 30–60 seconds while the device restarts."
        confirmLabel="Reboot Now"
        isPending={reboot.isPending}
        onConfirm={handleReboot}
      />
    </>
  );
}
