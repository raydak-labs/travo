// Connection source cards for the dashboard: one card per uplink, showing
// whether it is connected and the details of that uplink.

import { Cable, CheckCircle2, XCircle } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { StatusPill } from '@/components/ui/status-pill';

// ─── Connection source cards ──────────────────────────────────────────────────

export function SourceCard({
  title,
  icon: Icon,
  connected,
  children,
}: {
  title: string;
  icon: typeof Cable;
  connected: boolean;
  children?: React.ReactNode;
}) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <CardTitle className="flex items-center gap-2 text-gray-900 dark:text-slate-100">
            <Icon className="h-4 w-4 text-gray-500 dark:text-slate-400" />
            {title}
          </CardTitle>
          {connected ? (
            <CheckCircle2 className="h-4 w-4 text-[var(--status-ok-text)]" />
          ) : (
            <XCircle className="h-4 w-4 text-gray-400 dark:text-slate-600" />
          )}
        </div>
      </CardHeader>
      <CardContent>
        <div className="mb-3">
          <StatusPill tone={connected ? 'ok' : 'neutral'} withDot>
            {connected ? 'Connected' : 'Not connected'}
          </StatusPill>
        </div>
        {children && (
          <div className="space-y-2 text-xs text-gray-500 dark:text-gray-500">{children}</div>
        )}
      </CardContent>
    </Card>
  );
}

export function DetailRow({
  label,
  children,
  mono,
}: {
  label: string;
  children: React.ReactNode;
  mono?: boolean;
}) {
  return (
    <div className="flex flex-col gap-0.5 sm:flex-row sm:items-start sm:justify-between sm:gap-3">
      <span className="shrink-0">{label}</span>
      <span
        className={`min-w-0 break-words text-gray-700 dark:text-slate-300 sm:text-right ${mono ? 'font-mono' : ''}`}
      >
        {children}
      </span>
    </div>
  );
}
