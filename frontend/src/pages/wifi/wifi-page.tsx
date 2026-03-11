import { useState, useEffect } from 'react';
import {
  Wifi,
  WifiOff,
  Signal,
  Trash2,
  Radio,
  QrCode,
  Shuffle,
  RotateCcw,
  ShieldCheck,
} from 'lucide-react';
import { WifiQRDialog } from '@/components/wifi/wifi-qr-dialog';
import type { APConfig, GuestWifiConfig } from '@shared/index';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { CaptivePortalBanner } from '@/components/wifi/captive-portal-banner';
import { WifiModeCard } from '@/components/wifi/wifi-mode-card';
import { SignalStrengthIcon } from '@/components/wifi/signal-strength-icon';
import { SecurityBadge } from '@/components/wifi/security-badge';
import { WifiScanDialog } from './wifi-scan-dialog';
import { WifiHiddenNetworkDialog } from './wifi-hidden-network-dialog';
import {
  useWifiConnection,
  useWifiDisconnect,
  useSavedNetworks,
  useWifiDelete,
  useAPConfigs,
  useSetAPConfig,
  useMACAddresses,
  useSetMAC,
  useGuestWifi,
  useSetGuestWifi,
} from '@/hooks/use-wifi';

interface APFormState {
  ssid: string;
  encryption: string;
  key: string;
  enabled: boolean;
}

interface GuestFormState {
  enabled: boolean;
  ssid: string;
  encryption: string;
  key: string;
}

function generateRandomMAC(): string {
  const hex = () =>
    Math.floor(Math.random() * 256)
      .toString(16)
      .padStart(2, '0');
  const first = (Math.floor(Math.random() * 256) & 0xfe) | 0x02; // locally administered, unicast
  return [first.toString(16).padStart(2, '0'), hex(), hex(), hex(), hex(), hex()].join(':');
}

const MAC_REGEX = /^[0-9a-fA-F]{2}(:[0-9a-fA-F]{2}){5}$/;

export function WifiPage() {
  const { data: connection, isLoading: connectionLoading } = useWifiConnection();
  const { data: savedNetworks = [], isLoading: savedLoading } = useSavedNetworks();
  const { data: apConfigs, isLoading: apLoading } = useAPConfigs();
  const disconnectMutation = useWifiDisconnect();
  const deleteMutation = useWifiDelete();
  const setAP = useSetAPConfig();
  const { data: macAddresses, isLoading: macLoading } = useMACAddresses();
  const setMAC = useSetMAC();
  const { data: guestWifi, isLoading: guestLoading } = useGuestWifi();
  const setGuestWifi = useSetGuestWifi();
  const [apState, setApState] = useState<Record<string, APFormState>>({});
  const [qrAP, setQrAP] = useState<APConfig | null>(null);
  const [customMAC, setCustomMAC] = useState('');
  const [guestState, setGuestState] = useState<GuestFormState>({
    enabled: false,
    ssid: '',
    encryption: 'psk2',
    key: '',
  });

  useEffect(() => {
    if (apConfigs) {
      const state: Record<string, APFormState> = {};
      for (const ap of apConfigs) {
        state[ap.section] = {
          ssid: ap.ssid,
          encryption: ap.encryption,
          key: ap.key,
          enabled: ap.enabled,
        };
      }
      setApState(state);
    }
  }, [apConfigs]);

  useEffect(() => {
    if (macAddresses && macAddresses.length > 0) {
      setCustomMAC(macAddresses[0].custom_mac || '');
    }
  }, [macAddresses]);

  useEffect(() => {
    if (guestWifi) {
      setGuestState({
        enabled: guestWifi.enabled,
        ssid: guestWifi.ssid,
        encryption: guestWifi.encryption || 'psk2',
        key: guestWifi.key,
      });
    }
  }, [guestWifi]);

  return (
    <div className="space-y-6">
      <CaptivePortalBanner />

      {/* WiFi Mode */}
      <WifiModeCard />

      {/* Current Connection */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Current Connection</CardTitle>
          {connection?.connected ? (
            <Wifi className="h-4 w-4 text-green-500" />
          ) : (
            <WifiOff className="h-4 w-4 text-gray-400" />
          )}
        </CardHeader>
        <CardContent>
          {connectionLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-4 w-1/2" />
            </div>
          ) : connection?.connected ? (
            <div className="space-y-2">
              <div className="flex items-center gap-2">
                <SignalStrengthIcon signalPercent={connection.signal_percent} />
                <span className="font-medium text-gray-900 dark:text-white">{connection.ssid}</span>
                <Badge variant="success">Connected</Badge>
              </div>
              <div className="flex flex-wrap gap-x-4 gap-y-1 text-sm text-gray-600 dark:text-gray-400">
                <div className="flex items-center gap-1">
                  <Signal className="h-3.5 w-3.5" />
                  <span>
                    {connection.signal_percent}% ({connection.signal_dbm} dBm)
                  </span>
                </div>
                <span>Mode: {connection.mode}</span>
                <span>IP: {connection.ip_address}</span>
                <SecurityBadge encryption={connection.encryption} />
              </div>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => disconnectMutation.mutate()}
                  disabled={disconnectMutation.isPending}
                >
                  {disconnectMutation.isPending ? 'Disconnecting...' : 'Disconnect'}
                </Button>
                <WifiScanDialog />
                <WifiHiddenNetworkDialog />
              </div>
            </div>
          ) : (
            <div className="space-y-3">
              <p className="text-sm text-gray-500">Not connected to any WiFi network</p>
              <div className="flex gap-2">
                <WifiScanDialog />
                <WifiHiddenNetworkDialog />
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Saved Networks */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">Saved Networks</CardTitle>
        </CardHeader>
        <CardContent>
          {savedLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : savedNetworks.length === 0 ? (
            <p className="text-sm text-gray-500">No saved networks</p>
          ) : (
            <ul className="divide-y divide-gray-200 dark:divide-gray-800" role="list">
              {savedNetworks.map((network) => (
                <li key={network.section} className="flex items-center justify-between py-3">
                  <div className="flex items-center gap-3">
                    <Wifi className="h-4 w-4 text-gray-400" />
                    <div>
                      <p className="text-sm font-medium text-gray-900 dark:text-white">
                        {network.ssid}
                      </p>
                      <SecurityBadge encryption={network.encryption} />
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <Badge variant={network.auto_connect ? 'success' : 'outline'}>
                      {network.auto_connect ? 'Auto' : 'Manual'}
                    </Badge>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => deleteMutation.mutate(network.section)}
                      disabled={deleteMutation.isPending}
                      title="Remove network"
                    >
                      <Trash2 className="h-4 w-4 text-red-500" />
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      {/* Access Point Configuration */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">Access Point Configuration</CardTitle>
        </CardHeader>
        <CardContent>
          {apLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : !apConfigs || apConfigs.length === 0 ? (
            <p className="text-sm text-gray-500">No access point radios detected</p>
          ) : (
            <div className="space-y-6">
              {apConfigs.map((ap) => {
                const form = apState[ap.section];
                if (!form) return null;
                const bandLabel =
                  ap.band === '5g' ? '5 GHz' : ap.band === '2g' ? '2.4 GHz' : ap.band;
                return (
                  <div key={ap.section} className="space-y-3 rounded-lg border p-4">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <Radio className="h-4 w-4 text-gray-500" />
                        <span className="text-sm font-medium text-gray-900 dark:text-white">
                          {ap.radio}
                        </span>
                        <Badge variant="outline">{bandLabel}</Badge>
                        <span className="text-xs text-gray-500">Ch {ap.channel}</span>
                      </div>
                      <Switch
                        id={`ap-enabled-${ap.section}`}
                        label="Enabled"
                        checked={form.enabled}
                        onChange={(e) =>
                          setApState((prev) => ({
                            ...prev,
                            [ap.section]: { ...prev[ap.section], enabled: e.target.checked },
                          }))
                        }
                      />
                    </div>
                    <div className="space-y-2">
                      <label
                        htmlFor={`ap-ssid-${ap.section}`}
                        className="text-xs font-medium text-gray-600 dark:text-gray-400"
                      >
                        SSID
                      </label>
                      <Input
                        id={`ap-ssid-${ap.section}`}
                        value={form.ssid}
                        onChange={(e) =>
                          setApState((prev) => ({
                            ...prev,
                            [ap.section]: { ...prev[ap.section], ssid: e.target.value },
                          }))
                        }
                        placeholder="Network name"
                      />
                    </div>
                    <div className="space-y-2">
                      <label
                        htmlFor={`ap-enc-${ap.section}`}
                        className="text-xs font-medium text-gray-600 dark:text-gray-400"
                      >
                        Encryption
                      </label>
                      <Select
                        value={form.encryption}
                        onValueChange={(val) =>
                          setApState((prev) => ({
                            ...prev,
                            [ap.section]: {
                              ...prev[ap.section],
                              encryption: val,
                              key: val === 'none' ? '' : prev[ap.section].key,
                            },
                          }))
                        }
                      >
                        <SelectTrigger id={`ap-enc-${ap.section}`}>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="none">None (Open)</SelectItem>
                          <SelectItem value="psk2">WPA2-PSK</SelectItem>
                          <SelectItem value="sae">WPA3-SAE</SelectItem>
                          <SelectItem value="psk-mixed">WPA2/WPA3 Mixed</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                    {form.encryption !== 'none' && (
                      <div className="space-y-2">
                        <label
                          htmlFor={`ap-key-${ap.section}`}
                          className="text-xs font-medium text-gray-600 dark:text-gray-400"
                        >
                          Password
                        </label>
                        <Input
                          id={`ap-key-${ap.section}`}
                          type="password"
                          value={form.key}
                          onChange={(e) =>
                            setApState((prev) => ({
                              ...prev,
                              [ap.section]: { ...prev[ap.section], key: e.target.value },
                            }))
                          }
                          placeholder="Minimum 8 characters"
                        />
                      </div>
                    )}
                    <div className="flex gap-2">
                      <Button
                        size="sm"
                        disabled={setAP.isPending}
                        onClick={() =>
                          setAP.mutate({
                            section: ap.section,
                            config: {
                              ...ap,
                              ssid: form.ssid,
                              encryption: form.encryption,
                              key: form.key,
                              enabled: form.enabled,
                            },
                          })
                        }
                      >
                        {setAP.isPending ? 'Saving...' : 'Save'}
                      </Button>
                      <Button variant="outline" size="sm" onClick={() => setQrAP(ap)}>
                        <QrCode className="h-4 w-4 mr-1" />
                        QR Code
                      </Button>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>

      <WifiQRDialog
        open={qrAP !== null}
        onOpenChange={(open) => !open && setQrAP(null)}
        ap={qrAP}
      />

      {/* Guest Network */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Guest Network</CardTitle>
          <ShieldCheck className="h-4 w-4 text-gray-400" />
        </CardHeader>
        <CardContent>
          {guestLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div className="space-y-0.5">
                  <span className="text-sm font-medium text-gray-900 dark:text-white">
                    Enable Guest WiFi
                  </span>
                  <p className="text-xs text-gray-500">
                    Separate network (192.168.2.0/24) with client isolation
                  </p>
                </div>
                <Switch
                  id="guest-enabled"
                  label="Enabled"
                  checked={guestState.enabled}
                  onChange={(e) =>
                    setGuestState((prev) => ({ ...prev, enabled: e.target.checked }))
                  }
                />
              </div>
              {guestState.enabled && (
                <div className="space-y-3 rounded-lg border p-4">
                  <div className="space-y-2">
                    <label
                      htmlFor="guest-ssid"
                      className="text-xs font-medium text-gray-600 dark:text-gray-400"
                    >
                      SSID
                    </label>
                    <Input
                      id="guest-ssid"
                      value={guestState.ssid}
                      onChange={(e) => setGuestState((prev) => ({ ...prev, ssid: e.target.value }))}
                      placeholder="Guest network name"
                    />
                  </div>
                  <div className="space-y-2">
                    <label
                      htmlFor="guest-encryption"
                      className="text-xs font-medium text-gray-600 dark:text-gray-400"
                    >
                      Encryption
                    </label>
                    <Select
                      value={guestState.encryption}
                      onValueChange={(val) =>
                        setGuestState((prev) => ({
                          ...prev,
                          encryption: val,
                          key: val === 'none' ? '' : prev.key,
                        }))
                      }
                    >
                      <SelectTrigger id="guest-encryption">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="none">None (Open)</SelectItem>
                        <SelectItem value="psk2">WPA2-PSK</SelectItem>
                        <SelectItem value="sae">WPA3-SAE</SelectItem>
                        <SelectItem value="psk-mixed">WPA2/WPA3 Mixed</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                  {guestState.encryption !== 'none' && (
                    <div className="space-y-2">
                      <label
                        htmlFor="guest-key"
                        className="text-xs font-medium text-gray-600 dark:text-gray-400"
                      >
                        Password
                      </label>
                      <Input
                        id="guest-key"
                        type="password"
                        value={guestState.key}
                        onChange={(e) =>
                          setGuestState((prev) => ({ ...prev, key: e.target.value }))
                        }
                        placeholder="Minimum 8 characters"
                      />
                    </div>
                  )}
                  <p className="text-xs text-gray-500">
                    Client isolation is enabled — guests cannot see each other. Internet access
                    only, no LAN access.
                  </p>
                </div>
              )}
              <Button
                size="sm"
                disabled={setGuestWifi.isPending}
                onClick={() => setGuestWifi.mutate(guestState as GuestWifiConfig)}
              >
                {setGuestWifi.isPending ? 'Saving...' : 'Save'}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      {/* MAC Address Cloning */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">MAC Address Cloning</CardTitle>
        </CardHeader>
        <CardContent>
          {macLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : !macAddresses || macAddresses.length === 0 ? (
            <p className="text-sm text-gray-500">No STA interface detected</p>
          ) : (
            <div className="space-y-4">
              {macAddresses.map((mac) => (
                <div key={mac.interface} className="space-y-3">
                  <div className="flex items-center gap-2">
                    <Badge variant="outline">STA Interface</Badge>
                    {mac.current_mac && (
                      <span className="text-sm font-mono text-gray-600 dark:text-gray-400">
                        Current: {mac.current_mac}
                      </span>
                    )}
                  </div>
                  {mac.custom_mac && (
                    <p className="text-xs text-amber-600 dark:text-amber-400">
                      Custom MAC active: {mac.custom_mac}
                    </p>
                  )}
                  <div className="space-y-2">
                    <label
                      htmlFor="mac-input"
                      className="text-xs font-medium text-gray-600 dark:text-gray-400"
                    >
                      Custom MAC Address
                    </label>
                    <Input
                      id="mac-input"
                      value={customMAC}
                      onChange={(e) => setCustomMAC(e.target.value)}
                      placeholder="AA:BB:CC:DD:EE:FF"
                      className="font-mono"
                    />
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <Button size="sm" onClick={() => setCustomMAC(generateRandomMAC())}>
                      <Shuffle className="h-4 w-4 mr-1" />
                      Random
                    </Button>
                    <Button
                      size="sm"
                      disabled={
                        setMAC.isPending || (customMAC !== '' && !MAC_REGEX.test(customMAC))
                      }
                      onClick={() => setMAC.mutate(customMAC)}
                    >
                      {setMAC.isPending ? 'Applying...' : 'Apply'}
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={setMAC.isPending}
                      onClick={() => {
                        setCustomMAC('');
                        setMAC.mutate('');
                      }}
                    >
                      <RotateCcw className="h-4 w-4 mr-1" />
                      Reset to Default
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
