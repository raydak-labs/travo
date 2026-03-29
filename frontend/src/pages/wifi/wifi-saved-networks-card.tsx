import { Wifi, Trash2, ChevronUp, ChevronDown } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { SecurityBadge } from '@/components/wifi/security-badge';
import {
  useSavedNetworks,
  useWifiDelete,
  useSetNetworkPriority,
  useAutoReconnect,
  useSetAutoReconnect,
} from '@/hooks/use-wifi';

export function WifiSavedNetworksCard() {
  const { data: savedNetworks = [], isLoading: savedLoading } = useSavedNetworks();
  const deleteMutation = useWifiDelete();
  const priorityMutation = useSetNetworkPriority();
  const { data: autoReconnect } = useAutoReconnect();
  const setAutoReconnect = useSetAutoReconnect();

  return (
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
          <EmptyState message="No saved networks" />
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
  );
}
