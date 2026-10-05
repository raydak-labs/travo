import { useEffect, useState } from 'react';
import { Cable } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Button } from '@/components/ui/button';
import { QueryCard } from '@/components/ui/query-card';
import { Skeleton } from '@/components/ui/skeleton';
import { StatusPill, statusDotToneClass } from '@/components/ui/status-pill';
import { useUptimeLog } from '@/hooks/use-network';
import type { UptimeEvent } from '@shared/index';

/** Human duration from milliseconds. */
function formatDuration(ms: number): string {
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m`;
  const h = Math.floor(m / 60);
  const rem = m % 60;
  return rem > 0 ? `${h}h ${rem}m` : `${h}h`;
}

/** "4m ago", with the absolute time in the tooltip. */
function formatRelative(timestamp: number, now: number): string {
  const ms = now - timestamp;
  if (ms < 0) return 'just now';
  const mins = Math.floor(ms / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

/** Enough context to answer "has my connection been flaky?" without arithmetic. */
function summarize(events: readonly UptimeEvent[], now: number) {
  const windowStart = now - 24 * 60 * 60 * 1000;
  const inWindow = events.filter((e) => e.timestamp >= windowStart);
  const outages = inWindow.filter((e) => e.state !== 'connected').length;
  const connectedMs = inWindow.reduce((total, event, index) => {
    const next = events[index + 1];
    if (!next || event.state !== 'connected') return total;
    return total + (event.timestamp - next.timestamp);
  }, 0);
  const observedMs = Math.max(
    0,
    now - Math.min(now, inWindow[inWindow.length - 1]?.timestamp ?? now),
  );
  const availability = observedMs > 0 ? Math.round((connectedMs / observedMs) * 100) : null;
  return { outages, availability };
}

const VISIBLE_LIMIT = 10;

export function UptimeLogCard() {
  const { data: uptimeLog, isLoading, isError, error, refetch } = useUptimeLog();
  const [showAll, setShowAll] = useState(false);

  // A ticking clock so the "for 12m" and "4m ago" figures stay honest, at 30s
  // rather than per-second granularity.
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), 30_000);
    return () => window.clearInterval(id);
  }, []);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle>Connection Uptime Log</CardTitle>
        <Cable className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent>
        <QueryCard
          isLoading={isLoading}
          isError={isError}
          error={error}
          onRetry={() => void refetch()}
          loading={
            <div className="space-y-2">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          }
        >
          {!uptimeLog || uptimeLog.length === 0 ? (
            <EmptyState message="No connectivity events recorded yet" />
          ) : (
            <div className="space-y-3">
              <UptimeSummary events={uptimeLog} now={now} />
              <ol className="space-y-2">
                {(showAll ? uptimeLog : uptimeLog.slice(0, VISIBLE_LIMIT)).map((event, i) => {
                  const isConnected = event.state === 'connected';
                  // The newest entry describes the *ongoing* state, so its
                  // duration is now - timestamp rather than null.
                  const next = uptimeLog[i + 1];
                  const durationMs = next
                    ? event.timestamp - next.timestamp
                    : now - event.timestamp;
                  return (
                    <li key={event.timestamp} className="flex items-start gap-3 text-sm">
                      <span
                        aria-hidden="true"
                        className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${
                          isConnected ? statusDotToneClass.ok : statusDotToneClass.danger
                        }`}
                      />
                      <div className="flex flex-col gap-1">
                        <span
                          className={
                            isConnected
                              ? 'text-[var(--status-ok-text)]'
                              : 'text-[var(--status-danger-text)]'
                          }
                        >
                          {isConnected ? 'Connected' : 'Disconnected'}
                          {!next && ' (current)'}
                        </span>
                        <span className="text-xs text-gray-500 dark:text-gray-400">
                          <span title={new Date(event.timestamp).toLocaleString()}>
                            {formatRelative(event.timestamp, now)}
                          </span>
                          {durationMs > 0 && <> &mdash; lasted {formatDuration(durationMs)}</>}
                        </span>
                      </div>
                    </li>
                  );
                })}
              </ol>
              {uptimeLog.length > VISIBLE_LIMIT && (
                <Button variant="outline" size="sm" onClick={() => setShowAll((v) => !v)}>
                  {showAll ? 'Show recent only' : `Show all ${uptimeLog.length}`}
                </Button>
              )}
            </div>
          )}
        </QueryCard>
      </CardContent>
    </Card>
  );
}

function UptimeSummary({ events, now }: { events: readonly UptimeEvent[]; now: number }) {
  const { outages, availability } = summarize(events, now);
  const latest = events[0];
  const connected = latest?.state === 'connected';
  const currentState = connected ? 'Connected now' : 'Disconnected now';
  const currentFor = latest ? formatRelative(latest.timestamp, now) : null;
  // Text on a tinted surface takes the matching text token, which already
  // carries both themes.
  const toneText = connected ? 'text-[var(--status-ok-text)]' : 'text-[var(--status-danger-text)]';

  return (
    <div
      className={`rounded-md border p-3 text-sm ${
        connected
          ? 'border-[var(--status-ok-border)] bg-[var(--status-ok-surface)]'
          : 'border-[var(--status-danger-border)] bg-[var(--status-danger-surface)]'
      }`}
    >
      <div className="flex items-center gap-2">
        <StatusPill tone={connected ? 'ok' : 'danger'} withDot>
          {currentState}
        </StatusPill>
        {currentFor ? (
          <span className={toneText}>{`for ${formatDuration(now - latest.timestamp)}`}</span>
        ) : null}
      </div>
      <p className={`mt-1 text-xs ${toneText}`}>
        Last 24 hours: {outages} outage{outages === 1 ? '' : 's'}
        {availability !== null && ` \u00b7 ${availability}% available`}
      </p>
    </div>
  );
}
