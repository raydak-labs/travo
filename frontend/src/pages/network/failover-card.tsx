import { useMemo, useState } from 'react';
import { ArrowDown, ArrowUp, ArrowLeftRight } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { EmptyState } from '@/components/ui/empty-state';
import { useFailoverConfig, useSetFailoverConfig, useFailoverEvents } from '@/hooks/use-network';
import type { FailoverCandidate, FailoverConfig } from '@shared/index';

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
  return trackIPs.length > 0 ? trackIPs.join(', ') : '—';
}

export function FailoverCard() {
  const { data, isLoading } = useFailoverConfig();
  const { data: events = [] } = useFailoverEvents();
  const setConfig = useSetFailoverConfig();
  const [isEditing, setIsEditing] = useState(false);
  const [draft, setDraft] = useState<FailoverConfig | null>(null);

  const current = isEditing ? (draft ?? data) : data;
  const enabledCount = useMemo(
    () => current?.candidates.filter((candidate) => candidate.enabled).length ?? 0,
    [current],
  );

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">Connection Failover</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          <Skeleton className="h-4 w-1/2" />
          <Skeleton className="h-4 w-3/4" />
          <Skeleton className="h-10 w-28" />
        </CardContent>
      </Card>
    );
  }

  if (!current) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">Connection Failover</CardTitle>
        </CardHeader>
        <CardContent>
          <EmptyState
            message="Failover configuration is not available."
            icon={<ArrowLeftRight className="h-5 w-5" />}
          />
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

  const handleEdit = () => {
    if (data) {
      setDraft(cloneConfig(data));
    }
    setIsEditing(true);
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Connection Failover</CardTitle>
        <ArrowLeftRight className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent>
        {!isEditing ? (
          <div className="space-y-3">
            <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
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
                        {candidate.enabled ? candidate.tracking_state : 'excluded'}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
              <div className="mt-2 flex items-center justify-between">
                <span className="text-gray-500 dark:text-gray-400">Health targets</span>
                <span className="truncate pl-4 text-right">
                  {summarizeTrackIPs(current.health.track_ips)}
                </span>
              </div>
              <div className="mt-2 flex items-center justify-between">
                <span className="text-gray-500 dark:text-gray-400">Last event</span>
                <span>{events[0] ? new Date(events[0].timestamp).toLocaleString() : '—'}</span>
              </div>
            </div>

            {!current.service_installed ? (
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Install the `mwan3` service from the Services page before enabling ordered failover.
              </p>
            ) : null}

            <Button size="sm" onClick={handleEdit} disabled={setConfig.isPending}>
              Edit Failover Settings
            </Button>
          </div>
        ) : (
          <div className="space-y-4">
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
                <div key={candidate.interface_name} className="rounded-md border p-3">
                  <div className="flex items-start justify-between gap-3">
                    <div className="space-y-1">
                      <div className="font-medium">{candidate.label}</div>
                      <div className="text-sm text-gray-500 dark:text-gray-400">
                        Interface `{candidate.interface_name}` · {candidate.tracking_state}
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
                      <ArrowUp className="mr-1 h-4 w-4" />
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
                      <ArrowDown className="mr-1 h-4 w-4" />
                      Move down
                    </Button>
                  </div>
                </div>
              ))}
            </div>

            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-2 md:col-span-2">
                <label htmlFor="failover-track-ips" className="text-sm font-medium">
                  Health targets
                </label>
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

              {[
                ['interval', 'Interval (s)'],
                ['down', 'Failures before down'],
                ['up', 'Successes before recovery'],
              ].map(([field, label]) => (
                <div key={field} className="space-y-2">
                  <label htmlFor={field} className="text-sm font-medium">
                    {label}
                  </label>
                  <Input
                    id={field}
                    inputMode="numeric"
                    value={String(current.health[field as keyof typeof current.health] ?? '')}
                    disabled={setConfig.isPending}
                    onChange={(e) => {
                      const next = Number.parseInt(e.target.value, 10) || 0;
                      setDraft((prev) =>
                        prev
                          ? {
                              ...prev,
                              health: {
                                ...prev.health,
                                [field]: next,
                              },
                            }
                          : prev,
                      );
                    }}
                  />
                </div>
              ))}
            </div>

            {draft?.enabled && enabledCount === 0 ? (
              <p className="text-sm text-red-600 dark:text-red-400">
                Enable at least one uplink before turning automatic failover on.
              </p>
            ) : null}

            <div className="flex flex-wrap gap-2">
              <Button
                size="sm"
                onClick={handleSave}
                disabled={setConfig.isPending || ((draft?.enabled ?? false) && enabledCount === 0)}
              >
                {setConfig.isPending ? 'Saving…' : 'Save Failover Settings'}
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={handleCancel}
                disabled={setConfig.isPending}
              >
                Cancel
              </Button>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
