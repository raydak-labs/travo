import { RefreshCw, Shield, Power, Loader2, Wifi, WifiOff } from 'lucide-react';
import { Button } from '@/components/ui/button';

type QuickActionsButtonRowProps = {
  radioEnabled: boolean;
  setRadioPending: boolean;
  onToggleRadio: () => void;
  wifiRestarting: boolean;
  onRestartWifi: () => void;
  vpnTogglePending: boolean;
  desiredVpnEnabled: boolean | undefined;
  vpnEnabled: boolean;
  onToggleVpn: () => void;
  rebootPending: boolean;
  onOpenRebootDialog: () => void;
};

export function QuickActionsButtonRow({
  radioEnabled,
  setRadioPending,
  onToggleRadio,
  wifiRestarting,
  onRestartWifi,
  vpnTogglePending,
  desiredVpnEnabled,
  vpnEnabled,
  onToggleVpn,
  rebootPending,
  onOpenRebootDialog,
}: QuickActionsButtonRowProps) {
  return (
    <div className="flex flex-wrap gap-2">
      <Button variant="outline" size="sm" onClick={onToggleRadio} disabled={setRadioPending}>
        {setRadioPending ? (
          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        ) : radioEnabled ? (
          <Wifi className="mr-2 h-4 w-4" />
        ) : (
          <WifiOff className="mr-2 h-4 w-4" />
        )}
        {radioEnabled ? 'Disable WiFi' : 'Enable WiFi'}
      </Button>
      <Button variant="outline" size="sm" onClick={onRestartWifi} disabled={wifiRestarting}>
        {wifiRestarting ? (
          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        ) : (
          <RefreshCw className="mr-2 h-4 w-4" />
        )}
        Restart WiFi
      </Button>
      <Button variant="outline" size="sm" onClick={onToggleVpn} disabled={vpnTogglePending}>
        {vpnTogglePending ? (
          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        ) : (
          <Shield className="mr-2 h-4 w-4" />
        )}
        {vpnTogglePending
          ? desiredVpnEnabled
            ? 'Enabling VPN…'
            : 'Disabling VPN…'
          : vpnEnabled
            ? 'Disable VPN'
            : 'Enable VPN'}
      </Button>
      <Button
        variant="destructive"
        size="sm"
        onClick={onOpenRebootDialog}
        disabled={rebootPending}
      >
        {rebootPending ? (
          <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        ) : (
          <Power className="mr-2 h-4 w-4" />
        )}
        Reboot System
      </Button>
    </div>
  );
}
