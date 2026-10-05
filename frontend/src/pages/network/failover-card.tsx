import { useMemo, useState } from 'react';
import { ArrowDown, ArrowUp, ArrowLeftRight } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { CardInset } from '@/components/ui/card-inset';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { QueryCard } from '@/components/ui/query-card';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { EmptyState } from '@/components/ui/empty-state';
import { Label } from '@/components/ui/label';
import { useFailoverConfig, useSetFailoverConfig, useFailoverEvents } from '@/hooks/use-network';
import { useAlertStore } from '@/stores/alert-store';
import { OperatorEditOverwriteWarning } from './failover-operator-edit-warning';
import {
  OPERATOR_EDIT_OVERWRITE_ALERT,
  dismissedOverwriteAlerts,
  rememberDismissedOverwriteAlert,
} from './operator-edit-overwrite';
import type { FailoverCandidate, FailoverConfig, FailoverTrackingState } from '@shared/index';

/**
 * The tracking state as something a user can act on. `not_available` vs
 * `not_installed` is a distinction only the mwan3 author cares about, and
 * neither tells a user what to do.
 */
const TRACKING_LABEL: Record<FailoverTrackingState, string> = {
  online: 'Online',
  offline: 'Offline',
  disabled: 'Skipped',
  not_installed: 'Service missing',
  not_available: 'No device',
  unknown: 'Unknown',
};

function trackingLabel(candidate: FailoverCandidate): string {
  if (!candidate.enabled) return 'Skipped';
  return TRACKING_LABEL[candidate.tracking_state] ?? candidate.tracking_state;
}

function cloneConfig(config: FailoverConfig): FailoverConfig {
  return {
    ...config,
    candidates: config.candidates.map((candidate) => ({ ...candidate })),
    health: {
      ...config.health,
      track_ips: [...config.health.track_ips],
    },
    last_failover_event: config.last_failover_event ? { ...config.last_failover_event } : undefined,
  };
}

function moveCandidate(candidates: readonly FailoverCandidate[], index: number, direction: -1 | 1) {
  const next = candidates.map((candidate) => ({ ...candidate }));
  const target = index + direction;
  if (target < 0 || target >= next.length) {
    return next;
  }
  [next[index], next[target]] = [next[target], next[index]];
  return next.map((candidate, candidateIndex) => ({
    ...candidate,
    priority: candidateIndex + 1,
  }));
}

function summarizeTrackIPs(trackIPs: readonly string[]) {
  if (trackIPs.length === 0) return '—';
  if (trackIPs.length === 1) return trackIPs[0];
  return `${trackIPs.length} addresses`;
}

const HEALTH_FIELDS = [
  { field: 'interval', label: 'Interval (s)', min: 1 },
  { field: 'down', label: 'Failures before down', min: 1 },
  { field: 'up', label: 'Successes before recovery', min: 1 },
] as const;

export function FailoverCard() {
  const {
    data,
    isLoading,
    isError: failoverFailed,
    error: failoverError,
    refetch: refetchFailover,
  } = useFailoverConfig();
  const { data: events = [] } = useFailoverEvents();
  const setConfig = useSetFailoverConfig();
  const [isEditing, setIsEditing] = useState(false);
  const [draft, setDraft] = useState<FailoverConfig | null>(null);
  // Alerts of this type are the record a save left behind. Unlike a toast they
  // are still in the feed after a reload, so the operator is told again when
  // they come back to the card rather than exactly once.
  const alerts = useAlertStore((state) => state.alerts);
  // Dismissals survive the card unmounting: the feed does not, so a dismissal
  // held in component state would be lost on the next visit and the operator
  // would be shown — and have to clear — the same banner again.
  const [dismissedAlerts, setDismissedAlerts] =
    useState<readonly string[]>(dismissedOverwriteAlerts);

  // Memoised: the store holds a stable array but filter does not, and a fresh
  // array on every render would re-render this card without end.
  const latestOverwrite = useMemo(
    () =>
      alerts.find(
        (alert) =>
          alert.type === OPERATOR_EDIT_OVERWRITE_ALERT && !dismissedAlerts.includes(alert.id),
      ),
    [alerts, dismissedAlerts],
  );

  // The operator's own tuning is being replaced under their hands: say so on
  // the card they pressed Save on, and keep saying so while they edit it.
  const overwriteWarning = latestOverwrite ? (
    <OperatorEditOverwriteWarning
      alert={latestOverwrite}
      onDismiss={(id) => setDismissedAlerts(rememberDismissedOverwriteAlert(id))}
    />
  ) : null;

  // Held as raw text while editing: parsing on every keystroke turned a
  // cleared field into 0, which the backend rejects, so clearing a field and
  // saving produced a raw server error toast.
  const [healthDraft, setHealthDraft] = useState<Record<string, string>>({});

  const current = isEditing ? (draft ?? data) : data;
  const enabledCount = useMemo(
    () => current?.candidates.filter((candidate) => candidate.enabled).length ?? 0,
    [current],
  );

  if (!current) {
    return (
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle>Connection Failover</CardTitle>
          <ArrowLeftRight className="h-4 w-4 text-gray-500 dark:text-gray-400" />
        </CardHeader>
        <CardContent>
          <QueryCard
            isLoading={isLoading}
            isError={failoverFailed}
            error={failoverError}
            onRetry={() => void refetchFailover()}
            loading={
              <div className="space-y-2">
                <Skeleton className="h-4 w-1/2" />
                <Skeleton className="h-4 w-3/4" />
                <Skeleton className="h-10 w-28" />
              </div>
            }
          >
            <EmptyState
              message="Failover configuration is not available."
              icon={<ArrowLeftRight className="h-5 w-5" />}
            />
          </QueryCard>
        </CardContent>
      </Card>
    );
  }

  const handleSave = () => {
    if (!draft) {
      return;
    }
    setConfig.mutate(draft, {
      onSuccess: () => {
        setIsEditing(false);
      },
    });
  };

  const handleCancel = () => {
    if (data) {
      setDraft(cloneConfig(data));
    }
    setIsEditing(false);
  };

  const commitHealthField = (field: 'interval' | 'down' | 'up', raw: string) => {
    const parsed = Number.parseInt(raw, 10);
    if (!Number.isFinite(parsed)) return;
    setDraft((prev) => (prev ? { ...prev, health: { ...prev.health, [field]: parsed } } : prev));
  };

  const handleEdit = () => {
    if (data) {
      setDraft(cloneConfig(data));
    }
    setIsEditing(true);
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle>Connection Failover</CardTitle>
        <ArrowLeftRight className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent>
        {!isEditing ? (
          <div className="space-y-3">
            {overwriteWarning}
            <CardInset variant="muted">
              <div className="flex items-center justify-between">
                <span className="text-gray-500 dark:text-gray-400">Status</span>
                <span>
                  {!current.service_installed
                    ? 'Not installed'
                    : current.enabled
                      ? 'Enabled'
                      : 'Disabled'}
                </span>
              </div>
              <div className="mt-2 flex items-center justify-between">
                <span className="text-gray-500 dark:text-gray-400">Active uplink</span>
                <span>{current.active_interface || 'None'}</span>
              </div>
              <div className="mt-2">
                <div className="text-xs text-gray-500 dark:text-gray-400">Order</div>
                <div className="mt-1">
                  {current.candidates.map((candidate) => (
                    <div
                      key={candidate.interface_name}
                      className="flex items-center justify-between gap-3 py-1"
                    >
                      <span>
                        {candidate.priority}. {candidate.label}
                      </span>
                      <span className="text-sm text-gray-500 dark:text-gray-400">
                        <span title={candidate.tracking_state}>{trackingLabel(candidate)}</span>
                      </span>
                    </div>
                  ))}
                </div>
              </div>
              <div className="mt-2 flex items-center justify-between gap-3">
                <span
                  className="text-gray-500 dark:text-gray-400"
                  title="Addresses checked to decide whether the internet is really reachable"
                >
                  Addresses checked for internet
                </span>
                <span className="truncate pl-4 text-right">
                  {summarizeTrackIPs(current.health.track_ips)}
                </span>
              </div>
              <div className="mt-2 flex items-center justify-between">
                <span className="text-gray-500 dark:text-gray-400">Last event</span>
                <span>{events[0] ? new Date(events[0].timestamp).toLocaleString() : '—'}</span>
              </div>
            </CardInset>

            {!current.service_installed ? (
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Install the <code className="rounded bg-gray-100 px-1 dark:bg-gray-800">mwan3</code>{' '}
                service from the Services page before enabling ordered failover.
              </p>
            ) : null}

            <Button size="sm" onClick={handleEdit} disabled={setConfig.isPending}>
              Edit Failover Settings
            </Button>
          </div>
        ) : (
          <div className="space-y-4">
            {overwriteWarning}
            <Switch
              id="failover-enabled"
              label="Enable automatic failover"
              checked={draft?.enabled ?? false}
              disabled={!current.service_installed || setConfig.isPending}
              onChange={(e) =>
                setDraft((prev) => (prev ? { ...prev, enabled: e.currentTarget.checked } : prev))
              }
            />

            <div className="space-y-3">
              {draft?.candidates.map((candidate, index) => (
                <CardInset key={candidate.interface_name}>
                  <div className="flex items-start justify-between gap-3">
                    <div className="space-y-1">
                      <div className="font-medium">{candidate.label}</div>
                      <div className="text-sm text-gray-500 dark:text-gray-400">
                        Interface{' '}
                        <code className="rounded bg-gray-100 px-1 dark:bg-gray-800">
                          {candidate.interface_name}
                        </code>{' '}
                        · {trackingLabel(candidate)}
                      </div>
                    </div>
                    <Switch
                      id={`failover-candidate-${candidate.interface_name}`}
                      checked={candidate.enabled}
                      disabled={setConfig.isPending}
                      onChange={(e) =>
                        setDraft((prev) =>
                          prev
                            ? {
                                ...prev,
                                candidates: prev.candidates.map((item) =>
                                  item.interface_name === candidate.interface_name
                                    ? { ...item, enabled: e.currentTarget.checked }
                                    : item,
                                ),
                              }
                            : prev,
                        )
                      }
                    />
                  </div>

                  <div className="mt-3 flex flex-wrap gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      disabled={index === 0 || setConfig.isPending}
                      onClick={() =>
                        setDraft((prev) =>
                          prev
                            ? { ...prev, candidates: moveCandidate(prev.candidates, index, -1) }
                            : prev,
                        )
                      }
                    >
                      <ArrowUp className="h-4 w-4" />
                      Move up
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      disabled={
                        index === (draft?.candidates.length ?? 1) - 1 || setConfig.isPending
                      }
                      onClick={() =>
                        setDraft((prev) =>
                          prev
                            ? { ...prev, candidates: moveCandidate(prev.candidates, index, 1) }
                            : prev,
                        )
                      }
                    >
                      <ArrowDown className="h-4 w-4" />
                      Move down
                    </Button>
                  </div>
                </CardInset>
              ))}
            </div>

            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-2 md:col-span-2">
                <Label htmlFor="failover-track-ips" className="text-sm font-medium">
                  Health targets
                </Label>
                <Input
                  id="failover-track-ips"
                  value={draft?.health.track_ips.join(', ') ?? ''}
                  disabled={setConfig.isPending}
                  onChange={(e) =>
                    setDraft((prev) =>
                      prev
                        ? {
                            ...prev,
                            health: {
                              ...prev.health,
                              track_ips: e.target.value
                                .split(',')
                                .map((value) => value.trim())
                                .filter(Boolean),
                            },
                          }
                        : prev,
                    )
                  }
                />
              </div>

              {HEALTH_FIELDS.map(({ field, label, min }) => {
                const raw = healthDraft[field] ?? String(current.health[field] ?? '');
                const parsed = Number.parseInt(raw, 10);
                const invalid = raw !== '' && (!Number.isFinite(parsed) || parsed < min);
                const inputId = `failover-${field}`;
                return (
                  <div key={field} className="space-y-2">
                    <Label htmlFor={inputId} className="text-sm font-medium">
                      {label}
                    </Label>
                    <Input
                      id={inputId}
                      inputMode="numeric"
                      value={raw}
                      disabled={setConfig.isPending}
                      aria-invalid={invalid}
                      aria-describedby={invalid ? `${inputId}-error` : undefined}
                      onChange={(e) => setHealthDraft({ ...healthDraft, [field]: e.target.value })}
                      onBlur={() => commitHealthField(field, raw)}
                    />
                    {invalid && (
                      <p
                        id={`${inputId}-error`}
                        role="alert"
                        className="text-xs text-red-600 dark:text-red-400"
                      >
                        Enter a number of at least {min}.
                      </p>
                    )}
                  </div>
                );
              })}
            </div>

            {draft?.enabled && enabledCount === 0 ? (
              <p className="text-sm text-red-600 dark:text-red-400">
                Enable at least one uplink before turning automatic failover on.
              </p>
            ) : null}

            <div className="flex flex-wrap gap-2">
              <Button
                onClick={handleSave}
                disabled={setConfig.isPending || ((draft?.enabled ?? false) && enabledCount === 0)}
              >
                {setConfig.isPending ? 'Saving…' : 'Save Failover Settings'}
              </Button>
              <Button variant="outline" onClick={handleCancel} disabled={setConfig.isPending}>
                Cancel
              </Button>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
