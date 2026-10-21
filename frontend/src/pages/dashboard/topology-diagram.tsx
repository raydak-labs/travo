// Topology diagram for the dashboard: uplink sources on the left, the router
// in the middle, connected clients on the right.

import { Router, Wifi } from 'lucide-react';
import { Skeleton } from '@/components/ui/skeleton';
import type { SourceDef } from '@/hooks/use-topology-data';

// Diagram colours come from the status tokens so the topology reads as one
// language with the rest of the app instead of a private dark palette.
const LINK_UP = 'var(--status-ok-border)';
const LINK_IDLE = 'var(--status-neutral-border)';
const connectedDotClass = 'bg-[var(--status-ok-border)]';
const idleDotClass = 'bg-[var(--status-neutral-border)]';
const connectedIconClass = 'text-[var(--status-ok-text)]';
const idleIconClass = 'text-gray-500 dark:text-gray-500';

// ─── Topology diagram ────────────────────────────────────────────────────────

interface ClientDef {
  label: string;
  icon: typeof Wifi;
  count: number;
}

function TopologyRouterCenter({
  hostname,
  model,
  features,
  loading,
}: {
  hostname: string;
  model: string;
  features: { label: string; active: boolean }[];
  loading: boolean;
}) {
  if (loading) {
    return <Skeleton className="h-20 w-20 rounded-2xl" />;
  }

  return (
    <>
      <div className="relative flex h-20 w-20 items-center justify-center rounded-2xl border border-gray-300 dark:border-slate-600 bg-gray-100 dark:bg-slate-800">
        <Router className="h-10 w-10 text-gray-700 dark:text-slate-300" />
        <div className="absolute -right-1 -top-1 h-3 w-3 rounded-full bg-[var(--status-ok-border)] ring-2 ring-white dark:ring-gray-950" />
      </div>
      <div className="min-w-0 max-w-full px-1 text-center">
        <div className="text-xs font-semibold leading-tight text-gray-900 dark:text-gray-200">
          {hostname || 'OpenWRT'}
        </div>
        {model && (
          <div className="break-words text-[10px] leading-tight text-gray-500 dark:text-gray-500">
            {model}
          </div>
        )}
      </div>
      <div className="flex max-w-full flex-wrap justify-center gap-1">
        {features.map((f) => (
          <span
            key={f.label}
            className={`shrink-0 rounded border px-1.5 py-0.5 text-[10px] font-medium ${
              f.active
                ? 'border-[var(--status-ok-border)] bg-[var(--status-ok-surface)] text-[var(--status-ok-text)]'
                : 'border-[var(--status-neutral-border)] bg-[var(--status-neutral-surface)] text-[var(--status-neutral-text)]'
            }`}
          >
            {f.label}
          </span>
        ))}
      </div>
    </>
  );
}

export function TopologyDiagram({
  sources,
  clients,
  hostname,
  model,
  features,
  loading,
}: {
  sources: SourceDef[];
  clients: ClientDef[];
  hostname: string;
  model: string;
  features: { label: string; active: boolean }[];
  loading: boolean;
}) {
  const ROW_H = 56;
  const DIAGRAM_H = Math.max(sources.length, clients.length * 2) * ROW_H;
  const LEFT_COL = 172;
  const RIGHT_COL = 160;
  const CENTER_W = 160;

  const sourceYs = sources.map((_, i) => ((i + 0.5) / sources.length) * DIAGRAM_H);
  const clientYs =
    clients.length === 1
      ? [DIAGRAM_H / 2]
      : clients.map((_, i) => ((i + 0.5) / clients.length) * DIAGRAM_H);

  const routerCY = DIAGRAM_H / 2;

  return (
    <div className="@container overflow-visible rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-white/10 dark:bg-gray-950 sm:p-6">
      {/* Stack when topology card < ~576px (@xl container). Phones only; mid widths keep line graph. */}
      <div data-testid="topology-mobile" className="flex flex-col gap-5 @xl:hidden">
        <div className="space-y-2">
          {sources.map((src) => {
            const Icon = src.icon;
            return (
              <div key={src.label} className="flex min-w-0 items-center gap-2">
                <div
                  className={`h-2 w-2 flex-shrink-0 rounded-full ${
                    src.connected ? connectedDotClass : idleDotClass
                  }`}
                />
                <Icon
                  className={`h-4 w-4 flex-shrink-0 ${
                    src.connected ? connectedIconClass : idleIconClass
                  }`}
                />
                <div className="min-w-0">
                  <div className="text-xs font-medium leading-tight text-gray-900 dark:text-gray-200">
                    {src.label}
                  </div>
                  {src.detail && (
                    <div className="break-words text-[10px] leading-tight text-gray-500 dark:text-gray-500">
                      {src.detail}
                    </div>
                  )}
                </div>
              </div>
            );
          })}
        </div>

        <div className="flex flex-col items-center justify-center gap-2">
          <TopologyRouterCenter
            hostname={hostname}
            model={model}
            features={features}
            loading={loading}
          />
        </div>

        <div className="flex flex-wrap justify-center gap-4">
          {clients.map((client) => {
            const Icon = client.icon;
            return (
              <div key={client.label} className="flex items-center gap-2">
                <div className="rounded-lg border border-gray-200 dark:border-slate-700 bg-gray-100 dark:bg-slate-800 p-1.5">
                  <Icon className="h-4 w-4 text-gray-500 dark:text-slate-400" />
                </div>
                <div>
                  {loading ? (
                    <Skeleton className="h-5 w-8" />
                  ) : (
                    <div className="text-2xl font-bold leading-none text-gray-900 dark:text-slate-100">
                      {client.count}
                    </div>
                  )}
                  <div className="text-[10px] text-gray-500 dark:text-gray-500">{client.label}</div>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* Original horizontal line diagram once the card is wide enough */}
      <div
        data-testid="topology-desktop"
        className="relative hidden min-w-0 items-stretch overflow-x-auto @xl:flex"
        style={{ minHeight: DIAGRAM_H + 48 }}
      >
        <div style={{ width: LEFT_COL, minWidth: LEFT_COL }}>
          <div className="relative" style={{ height: DIAGRAM_H }}>
            {sources.map((src, i) => {
              const Icon = src.icon;
              const y = sourceYs[i];
              return (
                <div
                  key={src.label}
                  className="absolute flex items-center gap-2"
                  style={{ top: y - ROW_H / 2, height: ROW_H, left: 0, right: 0 }}
                >
                  <div
                    className={`h-2 w-2 flex-shrink-0 rounded-full ${
                      src.connected ? connectedDotClass : idleDotClass
                    }`}
                  />
                  <Icon
                    className={`h-4 w-4 flex-shrink-0 ${
                      src.connected ? connectedIconClass : idleIconClass
                    }`}
                  />
                  <div className="min-w-0">
                    <div className="text-xs font-medium leading-tight text-gray-900 dark:text-gray-200">
                      {src.label}
                    </div>
                    {src.detail && (
                      <div className="truncate text-[10px] leading-tight text-gray-500 dark:text-gray-500">
                        {src.detail}
                      </div>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        <div className="relative min-w-[40px] flex-1">
          <svg
            className="absolute inset-0 h-full w-full"
            preserveAspectRatio="none"
            viewBox={`0 0 100 ${DIAGRAM_H}`}
          >
            {sources.map((src, i) => {
              const y = sourceYs[i];
              const midX = 50;
              const color = src.connected ? LINK_UP : LINK_IDLE;
              return (
                <path
                  key={src.label}
                  d={`M 0 ${y} C ${midX} ${y}, ${midX} ${routerCY}, 100 ${routerCY}`}
                  fill="none"
                  stroke={color}
                  strokeWidth="1.5"
                  strokeDasharray={src.connected ? '0' : '4 3'}
                  opacity={src.connected ? 0.7 : 0.35}
                />
              );
            })}
          </svg>
        </div>

        <div
          className="flex w-[160px] flex-shrink-0 flex-col items-center justify-center gap-2"
          style={{ width: CENTER_W }}
        >
          <TopologyRouterCenter
            hostname={hostname}
            model={model}
            features={features}
            loading={loading}
          />
        </div>

        <div className="relative min-w-[40px] flex-1">
          <svg
            className="absolute inset-0 h-full w-full"
            preserveAspectRatio="none"
            viewBox={`0 0 100 ${DIAGRAM_H}`}
          >
            {clients.map((client, i) => {
              const y = clientYs[i];
              const midX = 50;
              const hasClients = client.count > 0;
              const color = hasClients ? LINK_UP : LINK_IDLE;
              return (
                <path
                  key={client.label}
                  d={`M 0 ${routerCY} C ${midX} ${routerCY}, ${midX} ${y}, 100 ${y}`}
                  fill="none"
                  stroke={color}
                  strokeWidth="1.5"
                  strokeDasharray={hasClients ? '0' : '4 3'}
                  opacity={hasClients ? 0.7 : 0.35}
                />
              );
            })}
          </svg>
        </div>

        <div style={{ width: RIGHT_COL, minWidth: RIGHT_COL }}>
          <div className="relative" style={{ height: DIAGRAM_H }}>
            {clients.map((client, i) => {
              const Icon = client.icon;
              const y = clientYs[i];
              return (
                <div
                  key={client.label}
                  className="absolute flex items-center gap-3"
                  style={{ top: y - ROW_H / 2, height: ROW_H, left: 0, right: 0 }}
                >
                  <div className="rounded-lg border border-gray-200 dark:border-slate-700 bg-gray-100 dark:bg-slate-800 p-1.5">
                    <Icon className="h-4 w-4 text-gray-500 dark:text-slate-400" />
                  </div>
                  <div>
                    {loading ? (
                      <Skeleton className="h-5 w-8" />
                    ) : (
                      <div className="text-2xl font-bold leading-none text-gray-900 dark:text-slate-100">
                        {client.count}
                      </div>
                    )}
                    <div className="text-[10px] text-gray-500 dark:text-gray-500">
                      {client.label}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </div>
  );
}
