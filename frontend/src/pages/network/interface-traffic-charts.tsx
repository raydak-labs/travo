import { useMemo } from 'react';
import { Activity } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { statusDotToneClass } from '@/components/ui/status-pill';
import { useWebSocket } from '@/hooks/use-websocket';
import { useTrafficHistory } from '@/hooks/use-data-usage';
import { interfaceNames, mergeInterfaceSeries } from '@/lib/traffic-series';
import { sortInterfaceNames } from './interface-traffic-utils';
import { InterfaceTrafficChartCard } from './interface-traffic-chart-card';

export function InterfaceTrafficCharts() {
  const { interfaceDataPoints, connected } = useWebSocket();
  // Retained server history, so the charts draw real traffic on a fresh page
  // load instead of sitting on "Collecting data…" until the live buffer fills.
  const { data: history } = useTrafficHistory();

  const names = useMemo(
    () => sortInterfaceNames(interfaceNames(history?.points, interfaceDataPoints)),
    [history, interfaceDataPoints],
  );

  const seriesFor = (name: string) =>
    mergeInterfaceSeries(history?.points ?? [], interfaceDataPoints[name] ?? [], name);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle>Interface Traffic</CardTitle>
        <div className="flex items-center gap-2">
          <span role="status" aria-live="polite" className="flex items-center gap-2">
            <span
              aria-hidden="true"
              className={`h-2 w-2 rounded-full ${
                connected ? statusDotToneClass.ok : statusDotToneClass.neutral
              }`}
            />
            <span className="sr-only">Live updates {connected ? 'connected' : 'disconnected'}</span>
          </span>
          <Activity className="h-4 w-4 text-gray-500 dark:text-gray-400" />
        </div>
      </CardHeader>
      <CardContent>
        {names.length === 0 ? (
          <div className="flex h-[100px] items-center justify-center text-sm text-gray-500 dark:text-gray-400">
            {connected
              ? 'Waiting for interface data…'
              : 'Live updates disconnected. Reconnect to resume charts.'}
          </div>
        ) : (
          <div className="grid gap-3 sm:grid-cols-2">
            {names.map((name) => (
              <InterfaceTrafficChartCard key={name} name={name} points={seriesFor(name)} />
            ))}
          </div>
        )}
        <div className="mt-3 flex justify-center gap-4 text-xs text-gray-500 dark:text-gray-400">
          <span className="flex items-center gap-1">
            <span
              aria-hidden="true"
              className="inline-block h-2 w-2 rounded-full"
              style={{ background: 'var(--chart-legend-rx)' }}
            />
            Download (RX)
          </span>
          <span className="flex items-center gap-1">
            <span
              aria-hidden="true"
              className="inline-block h-2 w-2 rounded-full"
              style={{ background: 'var(--chart-legend-tx)' }}
            />
            Upload (TX)
          </span>
        </div>
      </CardContent>
    </Card>
  );
}
