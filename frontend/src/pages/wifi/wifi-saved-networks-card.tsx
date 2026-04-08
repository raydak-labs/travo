import { useState } from 'react';
import { Wifi, Trash2, ChevronUp, ChevronDown, AlertTriangle } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
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
  const [pendingDelete, setPendingDelete] = useState<{ section: string; ssid: string } | null>(
    null,
  );

  const handleDelete = () => {
    if (pendingDelete) {
      deleteMutation.mutate(pendingDelete.section);
      setPendingDelete(null);
    }
  };

  const isRiskyDelete = pendingDelete && savedNetworks.length === 1 && !autoReconnect?.enabled;

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
                    onClick={() =>
                      setPendingDelete({ section: network.section, ssid: network.ssid })
                    }
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

        <Dialog
          open={pendingDelete !== null}
          onOpenChange={(open) => !open && setPendingDelete(null)}
        >
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Remove saved network</DialogTitle>
              <DialogDescription>
                Are you sure you want to remove the saved network{' '}
                <span className="font-medium text-gray-900 dark:text-white">
                  "{pendingDelete?.ssid}"
                </span>
                ?
              </DialogDescription>
            </DialogHeader>

            {isRiskyDelete && (
              <div className="flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 p-3 dark:border-amber-800 dark:bg-amber-950">
                <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600 dark:text-amber-400" />
                <p className="text-sm text-amber-800 dark:text-amber-200">
                  This is your only saved network and auto-reconnect is disabled. You will lose
                  automatic reconnection capability.
                </p>
              </div>
            )}

            <DialogFooter>
              <Button variant="outline" onClick={() => setPendingDelete(null)} type="button">
                Cancel
              </Button>
              <Button
                onClick={handleDelete}
                disabled={deleteMutation.isPending}
                variant="destructive"
                type="button"
              >
                {deleteMutation.isPending ? 'Removing...' : 'Remove network'}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </CardContent>
    </Card>
  );
}
