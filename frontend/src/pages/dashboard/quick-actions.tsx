import { useState } from 'react';
import { RefreshCw, Shield, Power, Loader2, Check, X, Wifi, WifiOff } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { useWifiDisconnect, useWifiConnect, useWifiConnection, useRadioStatus, useSetRadioEnabled } from '@/hooks/use-wifi';
import { useToggleWireguard, useVpnStatus } from '@/hooks/use-vpn';
import { useReboot } from '@/hooks/use-system';

type ActionState = 'idle' | 'loading' | 'success' | 'error';

export function QuickActions() {
  const [wifiState, setWifiState] = useState<ActionState>('idle');
  const [wifiRadioState, setWifiRadioState] = useState<ActionState>('idle');
  const [vpnState, setVpnState] = useState<ActionState>('idle');
  const [rebootState, setRebootState] = useState<ActionState>('idle');

  const { data: wifiConnection } = useWifiConnection();
  const { data: vpnStatusList } = useVpnStatus();
  const { data: radioStatus } = useRadioStatus();
  const wifiDisconnect = useWifiDisconnect();
  const wifiReconnect = useWifiConnect();
  const toggleWireguard = useToggleWireguard();
  const setRadioEnabled = useSetRadioEnabled();
  const reboot = useReboot();

  const resetState = (setter: (s: ActionState) => void) => {
    setTimeout(() => setter('idle'), 2000);
  };

  const handleRestartWifi = () => {
    if (wifiState === 'loading') return;
    setWifiState('loading');
    wifiDisconnect.mutate(undefined, {
      onSuccess: () => {
        if (wifiConnection?.ssid) {
          wifiReconnect.mutate(
            { ssid: wifiConnection.ssid, password: '' },
            {
              onSuccess: () => {
                setWifiState('success');
                resetState(setWifiState);
              },
              onError: () => {
                setWifiState('error');
                resetState(setWifiState);
              },
            },
          );
        } else {
          setWifiState('success');
          resetState(setWifiState);
        }
      },
      onError: () => {
        setWifiState('error');
        resetState(setWifiState);
      },
    });
  };

  const handleToggleVpn = () => {
    if (vpnState === 'loading') return;
    setVpnState('loading');
    const wg = vpnStatusList?.find((s) => s.type === 'wireguard');
    const newState = !(wg?.enabled ?? false);
    toggleWireguard.mutate(newState, {
      onSuccess: () => {
        setVpnState('success');
        resetState(setVpnState);
      },
      onError: () => {
        setVpnState('error');
        resetState(setVpnState);
      },
    });
  };

  const handleReboot = () => {
    if (rebootState === 'loading') return;
    if (!window.confirm('Are you sure you want to reboot the system?')) return;
    setRebootState('loading');
    reboot.mutate(undefined, {
      onSuccess: () => {
        setRebootState('success');
        resetState(setRebootState);
      },
      onError: () => {
        setRebootState('error');
        resetState(setRebootState);
      },
    });
  };

  const radioEnabled = radioStatus?.enabled ?? true;

  const handleToggleRadio = () => {
    if (wifiRadioState === 'loading') return;
    setWifiRadioState('loading');
    setRadioEnabled.mutate(!radioEnabled, {
      onSuccess: () => {
        setWifiRadioState('success');
        resetState(setWifiRadioState);
      },
      onError: () => {
        setWifiRadioState('error');
        resetState(setWifiRadioState);
      },
    });
  };

  function stateIcon(state: ActionState, defaultIcon: React.ReactNode) {
    switch (state) {
      case 'loading':
        return <Loader2 className="mr-2 h-4 w-4 animate-spin" />;
      case 'success':
        return <Check className="mr-2 h-4 w-4 text-green-500" />;
      case 'error':
        return <X className="mr-2 h-4 w-4 text-red-500" />;
      default:
        return defaultIcon;
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">Quick Actions</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={handleToggleRadio}
            disabled={wifiRadioState === 'loading'}
          >
            {stateIcon(
              wifiRadioState,
              radioEnabled ? (
                <Wifi className="mr-2 h-4 w-4" />
              ) : (
                <WifiOff className="mr-2 h-4 w-4" />
              ),
            )}
            {wifiRadioState === 'loading'
              ? 'Toggling...'
              : radioEnabled
                ? 'WiFi On'
                : 'WiFi Off'}
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={handleRestartWifi}
            disabled={wifiState === 'loading'}
          >
            {stateIcon(wifiState, <RefreshCw className="mr-2 h-4 w-4" />)}
            {wifiState === 'loading' ? 'Restarting...' : 'Restart WiFi'}
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={handleToggleVpn}
            disabled={vpnState === 'loading'}
          >
            {stateIcon(vpnState, <Shield className="mr-2 h-4 w-4" />)}
            {vpnState === 'loading' ? 'Toggling...' : 'Toggle VPN'}
          </Button>
          <Button
            variant="destructive"
            size="sm"
            onClick={handleReboot}
            disabled={rebootState === 'loading'}
          >
            {stateIcon(rebootState, <Power className="mr-2 h-4 w-4" />)}
            {rebootState === 'loading' ? 'Rebooting...' : 'Reboot System'}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
