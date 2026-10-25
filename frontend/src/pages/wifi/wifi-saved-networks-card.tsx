import { useState } from 'react';
import { Wifi, Trash2, ChevronUp, ChevronDown, AlertTriangle } from 'lucide-react';
import { EmptyState } from '@/components/ui/empty-state';
import { SectionHeading } from '@/components/ui/section-heading';
import { Badge, type BadgeProps } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
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
import { savedNetworkStatus, type SavedNetworkStatus } from './wifi-connect-utils';
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

const STATUS_BADGE = {
  connected: { label: 'Connected', variant: 'success' },
  'not-connected': { label: 'Not connected', variant: 'warning' },
  standby: { label: 'Standby', variant: 'secondary' },
} as const satisfies Record<
  SavedNetworkStatus,
  { label: string; variant: NonNullable<BadgeProps['variant']> }
>;

/** Saved station profiles, rendered as the second section of the uplink card. */
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

  const [pendingConnect, setPendingConnect] = useState<{
    ssid: string;
    encryption: string;
  } | null>(null);

  const handleDelete = () => {
    if (pendingDelete) {
      deleteMutation.mutate(pendingDelete.section);
      setPendingDelete(null);
    }
  };

  const isRiskyDelete = pendingDelete && savedNetworks.length === 1 && !autoReconnect?.enabled;
  // Deleting the profile currently in use drops the uplink.
  const isDeletingActive = pendingDelete !== null && connection?.ssid === pendingDelete.ssid;

  function confirmConnect() {
    const target = pendingConnect;
    if (!target) return;
    setPendingConnect(null);
    connectMutation.mutate({ ssid: target.ssid, password: '', encryption: target.encryption });
  }

  return (
    <section className="min-w-0" aria-label="Saved networks">
      <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <SectionHeading className="mb-0">Saved networks</SectionHeading>
            <Badge variant="secondary">{savedNetworks.length}</Badge>
            <InfoTooltip text={SAVED_LIST_TOOLTIP} />
          </div>
          <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
            Connect uses the stored password. Arrows change automatic priority.
          </p>
        </div>
        <div className="flex items-center gap-1">
          <Switch
            id="auto-reconnect"
            label="Auto-reconnect"
            checked={autoReconnect?.enabled ?? false}
            onChange={(e) => setAutoReconnect.mutate(e.target.checked)}
            disabled={setAutoReconnect.isPending}
          />
          <InfoTooltip text={AUTO_RECONNECT_HELP} />
        </div>
      </div>
      <div className="space-y-4">
        {savedLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-10 w-full" />
            <Skeleton className="h-10 w-full" />
          </div>
        ) : savedNetworks.length === 0 ? (
          <EmptyState message="No saved networks" />
        ) : (
          <div className="overflow-hidden rounded-lg border border-gray-200 dark:border-white/10">
            <div className="hidden grid-cols-[minmax(0,1fr)_6.5rem_5.5rem_auto] items-center gap-3 border-b border-gray-200 bg-gray-50 px-4 py-2 text-[11px] font-semibold uppercase tracking-wide text-gray-500 sm:grid dark:border-white/10 dark:bg-white/5 dark:text-gray-400">
              <span>Network</span>
              <span>Status</span>
              <span>Priority</span>
              <span className="text-right">Actions</span>
            </div>
            <ul className="divide-y divide-gray-200 dark:divide-white/10" role="list">
              {savedNetworks.map((network, index) => {
                const isThisNetwork =
                  connection?.connected === true && connection.ssid === network.ssid;
                const status = savedNetworkStatus(network, connection);
                const rowBusy =
                  connectMutation.isPending ||
                  priorityMutation.isPending ||
                  deleteMutation.isPending;
                return (
                  <li
                    key={network.section}
                    className="flex flex-col gap-3 px-4 py-3 sm:grid sm:grid-cols-[minmax(0,1fr)_6.5rem_5.5rem_auto] sm:items-center sm:gap-3"
                  >
                    <div className="flex min-w-0 items-center gap-3">
                      <Wifi className="h-4 w-4 shrink-0 text-gray-500 dark:text-gray-400" />
                      <div className="min-w-0">
                        <p className="truncate text-sm font-medium">{network.ssid}</p>
                        <SecurityBadge encryption={network.encryption} />
                      </div>
                    </div>
                    <div className="flex items-center gap-2 sm:contents">
                      <div className="justify-self-start">
                        <Badge variant={STATUS_BADGE[status].variant}>
                          {STATUS_BADGE[status].label}
                        </Badge>
                      </div>
                      <div className="flex items-center gap-1">
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
                          aria-label={`Move ${network.ssid} up`}
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
                          aria-label={`Move ${network.ssid} down`}
                        >
                          <ChevronDown className="h-4 w-4" />
                        </Button>
                      </div>
                      <div className="ml-auto flex items-center justify-end gap-1 sm:ml-0">
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
                          onClick={() => setPendingConnect(network)}
                        >
                          {connectMutation.isPending &&
                          connectMutation.variables?.ssid === network.ssid
                            ? 'Connecting…'
                            : 'Connect'}
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() =>
                            setPendingDelete({ section: network.section, ssid: network.ssid })
                          }
                          disabled={rowBusy}
                          title="Remove network"
                          aria-label={`Remove ${network.ssid}`}
                        >
                          <Trash2 className="h-4 w-4 text-red-500" />
                        </Button>
                      </div>
                    </div>
                  </li>
                );
              })}
            </ul>
          </div>
        )}

        <ConfirmDialog
          open={pendingConnect !== null}
          onOpenChange={(open) => !open && setPendingConnect(null)}
          title={`Switch to ${pendingConnect?.ssid ?? ''}?`}
          description="Connecting uses the saved password and can take up to 30 seconds."
          warningText={
            connection?.connected
              ? `The current link drops and internet moves to ${pendingConnect?.ssid ?? 'this network'}. If you are managing the router over WiFi you may lose access.`
              : undefined
          }
          confirmLabel="Connect"
          isPending={connectMutation.isPending}
          onConfirm={confirmConnect}
        />

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

            <div className="flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 p-3 dark:border-amber-800 dark:bg-amber-950">
              <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600 dark:text-amber-400" />
              <div className="space-y-1 text-sm text-amber-800 dark:text-amber-200">
                {/* The saved password is unrecoverable, so this always applies,
                    not only in the narrow last-network case. */}
                <p>
                  The saved password is deleted and cannot be recovered — you would have to re-enter
                  it to reconnect to this network.
                </p>
                {isDeletingActive && (
                  <p className="font-medium">
                    You are connected to this network right now. Removing it drops the uplink.
                  </p>
                )}
                {isRiskyDelete && (
                  <p>
                    This is your only saved network and auto-reconnect is disabled. You will lose
                    automatic reconnection capability.
                  </p>
                )}
              </div>
            </div>

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
      </div>
    </section>
  );
}
