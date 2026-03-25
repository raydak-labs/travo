import { useState } from 'react';
import {
  Wifi,
  WifiOff,
  Signal,
  Trash2,
  Radio,
  Cpu,
  ChevronUp,
  ChevronDown,
} from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
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
import { GuestNetworkCard } from './guest-network-card';
import { MACAddressCard } from './mac-address-card';
import { BandSwitchingCard } from './band-switching-card';
import { WiFiScheduleCard } from './wifi-schedule-card';
import { MACPolicyCard } from './mac-policy-card';
import { APConfigCard } from './ap-config-card';
import {
  useWifiConnection,
  useWifiDisconnect,
  useSavedNetworks,
  useWifiDelete,
  useSetNetworkPriority,
  useRadios,
  useSetRadioRole,
  useAutoReconnect,
  useSetAutoReconnect,
} from '@/hooks/use-wifi';

export function WifiPage() {
  const { data: connection, isLoading: connectionLoading } = useWifiConnection();
  const { data: savedNetworks = [], isLoading: savedLoading } = useSavedNetworks();
  const disconnectMutation = useWifiDisconnect();
  const deleteMutation = useWifiDelete();
  const priorityMutation = useSetNetworkPriority();
  const { data: radios, isLoading: radiosLoading } = useRadios();
  const setRadioRole = useSetRadioRole();
  const { data: autoReconnect } = useAutoReconnect();
  const setAutoReconnect = useSetAutoReconnect();
  const [advancedOpen, setAdvancedOpen] = useState(false);

  return (
    <div className="space-y-6">
      <CaptivePortalBanner />

      {/* WiFi Mode */}
      <WifiModeCard />

      {/* Radio Hardware */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Radio Hardware</CardTitle>
          <Cpu className="h-4 w-4 text-gray-400" />
        </CardHeader>
        <CardContent>
          {radiosLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : !radios || radios.length === 0 ? (
            <p className="text-sm text-gray-500">No radio hardware detected</p>
          ) : (
            <div className="space-y-3">
              {radios.map((radio) => {
                const bandLabel =
                  radio.band === '5g'
                    ? '5 GHz'
                    : radio.band === '2g'
                      ? '2.4 GHz'
                      : radio.band === '6g'
                        ? '6 GHz'
                        : radio.band;
                // Recommended: 5 GHz = AP, 2.4 GHz = STA (optimal travel router config)
                const recommendedRole = radio.band === '5g' ? 'ap' : radio.band === '2g' ? 'sta' : null;
                const isRecommended = recommendedRole && radio.role === recommendedRole;
                return (
                  <div
                    key={radio.name}
                    className="flex items-center justify-between rounded-lg border p-3 gap-3"
                  >
                    <div className="flex items-center gap-3 min-w-0">
                      <Radio className="h-4 w-4 shrink-0 text-gray-500" />
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <p className="text-sm font-medium text-gray-900 dark:text-white">
                            {radio.name}
                          </p>
                          {isRecommended && (
                            <Badge className="bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200 text-xs">
                              Recommended
                            </Badge>
                          )}
                          <Badge variant={radio.disabled ? 'destructive' : 'success'}>
                            {radio.disabled ? 'Disabled' : 'Active'}
                          </Badge>
                        </div>
                        <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-gray-500">
                          <span>{bandLabel}</span>
                          <span>Ch {radio.channel}</span>
                          <span>{radio.htmode}</span>
                          <span>{radio.type}</span>
                        </div>
                      </div>
                    </div>
                    <Select
                      value={radio.role}
                      onValueChange={(role) => setRadioRole.mutate({ name: radio.name, role })}
                      disabled={setRadioRole.isPending}
                    >
                      <SelectTrigger className="w-32 shrink-0">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="ap">AP only</SelectItem>
                        <SelectItem value="sta">STA only</SelectItem>
                        <SelectItem value="both">Both (repeater)</SelectItem>
                        <SelectItem value="none">Disabled</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>

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
          <div className="mb-4 flex items-center justify-between rounded-lg border p-3">
            <div className="space-y-0.5">
              <span className="text-sm font-medium text-gray-900 dark:text-white">
                Auto-Reconnect
              </span>
              <p className="text-xs text-gray-500">
                Automatically reconnect to saved networks when connection drops
              </p>
            </div>
            <Switch
              id="auto-reconnect"
              label="Auto-reconnect"
              checked={autoReconnect?.enabled ?? false}
              onChange={(e) => setAutoReconnect.mutate(e.target.checked)}
              disabled={setAutoReconnect.isPending}
            />
          </div>
          {savedLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : savedNetworks.length === 0 ? (
            <p className="text-sm text-gray-500">No saved networks</p>
          ) : (
            <ul className="divide-y divide-gray-200 dark:divide-gray-800" role="list">
              {savedNetworks.map((network, index) => (
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
                  <div className="flex items-center gap-1">
                    <Badge variant={network.auto_connect ? 'success' : 'outline'}>
                      {network.auto_connect ? 'Auto' : 'Manual'}
                    </Badge>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => {
                        const ssids = savedNetworks.map((n) => n.ssid);
                        const newSsids = [...ssids];
                        [newSsids[index - 1], newSsids[index]] = [
                          newSsids[index],
                          newSsids[index - 1],
                        ];
                        priorityMutation.mutate({ ssids: newSsids });
                      }}
                      disabled={index === 0 || priorityMutation.isPending}
                      title="Move up (higher priority)"
                    >
                      <ChevronUp className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => {
                        const ssids = savedNetworks.map((n) => n.ssid);
                        const newSsids = [...ssids];
                        [newSsids[index], newSsids[index + 1]] = [
                          newSsids[index + 1],
                          newSsids[index],
                        ];
                        priorityMutation.mutate({ ssids: newSsids });
                      }}
                      disabled={index === savedNetworks.length - 1 || priorityMutation.isPending}
                      title="Move down (lower priority)"
                    >
                      <ChevronDown className="h-4 w-4" />
                    </Button>
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

      <APConfigCard />

      {/* Advanced Settings (collapsible) */}
      <div>
        <button
          className="flex w-full items-center justify-between rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 text-sm font-medium text-gray-700 hover:bg-gray-100 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-300 dark:hover:bg-gray-800"
          onClick={() => setAdvancedOpen((o) => !o)}
          aria-expanded={advancedOpen}
        >
          <span>Advanced Settings</span>
          <ChevronDown
            className={`h-4 w-4 text-gray-500 transition-transform ${advancedOpen ? 'rotate-180' : ''}`}
          />
        </button>
        {advancedOpen && (
          <div className="mt-4 space-y-4">
            <GuestNetworkCard />
            <MACAddressCard />
            <MACPolicyCard />
            <BandSwitchingCard />
            <WiFiScheduleCard />
          </div>
        )}
      </div>
    </div>
  );
}
