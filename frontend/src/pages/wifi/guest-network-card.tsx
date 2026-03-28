import { useState, useEffect } from 'react';
import { ShieldCheck } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
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
import { useGuestWifi, useSetGuestWifi } from '@/hooks/use-wifi';
import type { GuestWifiConfig } from '@shared/index';

interface GuestFormState {
  enabled: boolean;
  ssid: string;
  encryption: string;
  key: string;
}

export function GuestNetworkCard() {
  const { data: guestWifi, isLoading: guestLoading } = useGuestWifi();
  const setGuestWifi = useSetGuestWifi();
  const [guestState, setGuestState] = useState<GuestFormState>({
    enabled: false,
    ssid: '',
    encryption: 'psk2',
    key: '',
  });

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
                onChange={(e) => setGuestState((prev) => ({ ...prev, enabled: e.target.checked }))}
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
                      onChange={(e) => setGuestState((prev) => ({ ...prev, key: e.target.value }))}
                      placeholder="Minimum 8 characters"
                    />
                  </div>
                )}
                <p className="text-xs text-gray-500">
                  Client isolation is enabled — guests cannot see each other. Internet access only,
                  no LAN access.
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
  );
}
