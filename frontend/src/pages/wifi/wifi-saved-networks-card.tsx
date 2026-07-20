import { useState } from 'react';
import { Wifi, Trash2, ChevronUp, ChevronDown, AlertTriangle } from 'lucide-react';
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
} from '@/components/ui/card';
import { CardInset } from '@/components/ui/card-inset';
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
import { InfoTooltip } from '@/components/ui/info-tooltip';
import { SecurityBadge } from '@/components/wifi/security-badge';
import {
  useSavedNetworks,
  useWifiDelete,
  useSetNetworkPriority,
  useAutoReconnect,
  useSetAutoReconnect,
  useWifiConnect,
  useWifiConnection,
} from '@/hooks/use-wifi';

const SAVED_LIST_TOOLTIP =
  'Priority is used when the router must pick a saved profile—for example when you turn on Wi‑Fi client mode, or if more than one STA profile needs reconciling. It is not continuous roaming by signal strength. Use Connect to switch to a standby network using its saved password.';

const AUTO_RECONNECT_HELP =
  'If Wi‑Fi drops, periodically runs a safe reconnect for the active profile. It does not automatically switch to another saved SSID for stronger signal.';

export function WifiSavedNetworksCard() {
  const { data: savedNetworks = [], isLoading: savedLoading } = useSavedNetworks();
  const { data: connection } = useWifiConnection();
  const connectMutation = useWifiConnect();
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
    <Card className="flex h-full min-w-0 flex-col">
      <CardHeader className="space-y-1">
        <div className="flex items-center gap-2">
          <CardTitle>Saved networks</CardTitle>
          <InfoTooltip text={SAVED_LIST_TOOLTIP} />
        </div>
        <CardDescription>
          In use = enabled profile; standby = saved but not active. <strong>Connect</strong> uses
          the stored password. Arrows set priority for automatic selection (see info).
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-1 flex-col space-y-4">
        <CardInset
          variant="muted"
          className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between"
        >
          <div className="min-w-0 space-y-0.5 pr-2">
            <div className="flex items-center gap-2">
              <span className="text-sm font-medium">Auto-reconnect</span>
              <InfoTooltip text={AUTO_RECONNECT_HELP} />
            </div>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Retry Wi‑Fi when the link drops (active profile only).
            </p>
          </div>
          <Switch
            id="auto-reconnect"
            label="Auto-reconnect"
            checked={autoReconnect?.enabled ?? false}
            onChange={(e) => setAutoReconnect.mutate(e.target.checked)}
            disabled={setAutoReconnect.isPending}
            className="shrink-0 sm:ml-auto"
          />
        </CardInset>

        {savedLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : savedNetworks.length === 0 ? (
          <EmptyState message="No saved networks" />
        ) : (
          <ul
            className="divide-y divide-gray-200 rounded-lg border border-gray-200 dark:divide-white/10 dark:border-white/10"
            role="list"
          >
            {savedNetworks.map((network, index) => {
              const isThisNetwork =
                connection?.connected === true && connection.ssid === network.ssid;
              const rowBusy =
                connectMutation.isPending ||
                priorityMutation.isPending ||
                deleteMutation.isPending;
              return (
              <li
                key={network.section}
                className="flex flex-col gap-3 py-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4 sm:px-1"
              >
                <div className="flex min-w-0 items-center gap-3">
                  <Wifi className="h-4 w-4 shrink-0 text-gray-500 dark:text-gray-400" />
                  <div className="min-w-0">
                    <p className="truncate text-sm font-medium">{network.ssid}</p>
                    <SecurityBadge encryption={network.encryption} />
                  </div>
                </div>
                <div className="flex shrink-0 flex-wrap items-center justify-end gap-1 sm:gap-2">
                  <Badge variant={network.auto_connect ? 'success' : 'secondary'}>
                    {network.auto_connect ? 'In use' : 'Standby'}
                  </Badge>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    disabled={isThisNetwork || rowBusy}
                    title={
                      isThisNetwork
                        ? 'Already connected to this network'
                        : 'Connect using saved password'
                    }
                    onClick={() =>
                      connectMutation.mutate({
                        ssid: network.ssid,
                        password: '',
                        encryption: network.encryption,
                      })
                    }
                  >
                    {connectMutation.isPending &&
                    connectMutation.variables?.ssid === network.ssid
                      ? 'Connecting…'
                      : 'Connect'}
                  </Button>
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
                    disabled={index === 0 || rowBusy}
                    title="Higher priority when the router picks among saved profiles"
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
                    disabled={index === savedNetworks.length - 1 || rowBusy}
                    title="Lower priority"
                  >
                    <ChevronDown className="h-4 w-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() =>
                      setPendingDelete({ section: network.section, ssid: network.ssid })
                    }
                    disabled={rowBusy}
                    title="Remove network"
                  >
                    <Trash2 className="h-4 w-4 text-red-500" />
                  </Button>
                </div>
              </li>
              );
            })}
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
                <span className="font-medium">"{pendingDelete?.ssid}"</span>?
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
