import { useMemo } from 'react';
import { AreaChart, Area, XAxis, YAxis, ResponsiveContainer, Tooltip } from 'recharts';
import { ArrowDownToLine, ArrowUpFromLine, RefreshCw } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { useWebSocket, type InterfaceDataPoint } from '@/hooks/use-websocket';
import { useTopologyData } from '@/hooks/use-topology-data';
import { formatRate } from '@/lib/utils';
import { computeNetworkRates } from '@/pages/dashboard/network-chart-utils';
import { uplinkInterfaceName } from '@/lib/uplink';

/** Adapts a per-interface series to the shape `computeNetworkRates` expects. */
function toRatePoints(series: InterfaceDataPoint[]) {
  return computeNetworkRates(
    series.map((p) => ({
      timestamp: p.timestamp,
      cpu: 0,
      memoryUsed: 0,
      memoryTotal: 0,
      rxBytes: p.rxBytes,
      txBytes: p.txBytes,
    })),
  );
}

export function NetworkChart() {
  const { interfaceDataPoints, connected } = useWebSocket();
  const { wan, wanMedium } = useTopologyData();

  // Chart the uplink that is actually carrying the internet. Previously this
  // took `msg.network[0]`, which is always `br-lan`.
  const uplinkName = uplinkInterfaceName(
    Object.keys(interfaceDataPoints),
    wan?.name ?? null,
  );
  const series = uplinkName ? interfaceDataPoints[uplinkName] : undefined;

  const chartData = useMemo(() => toRatePoints(series ?? []), [series]);

  const latestRx = chartData.length > 0 ? chartData[chartData.length - 1].rx : 0;
  const latestTx = chartData.length > 0 ? chartData[chartData.length - 1].tx : 0;

  const mediumLabel =
    wanMedium === 'wifi' ? 'WiFi uplink' : wanMedium === 'usb' ? 'USB uplink' : 'uplink';

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle>
          Internet Throughput
          {uplinkName && (
            <span className="ml-2 text-xs font-normal text-gray-500 dark:text-gray-400">
              {uplinkName}
            </span>
          )}
        </CardTitle>
        <span
          role="status"
          aria-live="polite"
          className="flex items-center gap-2"
        >
          <span
            aria-hidden="true"
            className={`h-2 w-2 rounded-full ${
              connected ? 'bg-emerald-500 dark:bg-emerald-400' : 'bg-gray-400 dark:bg-gray-600'
            }`}
          />
          <span className="sr-only">
            Live updates {connected ? 'connected' : 'disconnected'}
          </span>
        </span>
      </CardHeader>
      <CardContent>
        {chartData.length < 2 ? (
          <div className="flex h-[120px] flex-col items-center justify-center gap-2 text-sm text-gray-500 dark:text-gray-400">
            {!connected ? (
              <>
                <span>Live updates disconnected.</span>
                <Button variant="outline" size="sm" onClick={() => window.location.reload()}>
                  <RefreshCw className="h-3.5 w-3.5" />
                  Reconnect
                </Button>
              </>
            ) : (
              <span>Collecting data…</span>
            )}
          </div>
        ) : (
          <ResponsiveContainer width="100%" height={120}>
            <AreaChart data={chartData} margin={{ top: 5, right: 5, bottom: 0, left: -20 }}>
              <defs>
                <linearGradient id="rxGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="var(--chart-rx)" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="var(--chart-rx)" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="txGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="var(--chart-tx)" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="var(--chart-tx)" stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis
                dataKey="time"
                tick={{ fontSize: 10, fill: 'var(--chart-axis)' }}
                stroke="var(--chart-grid)"
                tickLine={false}
                axisLine={false}
              />
              <YAxis
                tick={{ fontSize: 10, fill: 'var(--chart-axis)' }}
                stroke="var(--chart-grid)"
                tickLine={false}
                axisLine={false}
                tickFormatter={(v: number) => formatRate(v)}
              />
              <Tooltip
                contentStyle={{
                  backgroundColor: 'var(--chart-tooltip-bg)',
                  border: '1px solid var(--chart-tooltip-border)',
                  borderRadius: '6px',
                  color: 'var(--chart-tooltip-text)',
                  fontSize: '12px',
                }}
                formatter={(value) => formatRate(Number(value ?? 0))}
              />
              <Area
                type="monotone"
                dataKey="rx"
                stroke="var(--chart-rx)"
                fill="url(#rxGrad)"
                strokeWidth={1.5}
                name="Download"
              />
              <Area
                type="monotone"
                dataKey="tx"
                stroke="var(--chart-tx)"
                fill="url(#txGrad)"
                strokeWidth={1.5}
                name="Upload"
              />
            </AreaChart>
          </ResponsiveContainer>
        )}
        <div className="mt-2 flex justify-center gap-4 text-xs text-gray-500 dark:text-gray-400">
          <span className="flex items-center gap-1">
            <ArrowDownToLine
              className="h-3 w-3"
              style={{ color: 'var(--chart-legend-rx)' }}
              aria-hidden="true"
            />
            RX {formatRate(latestRx)}
          </span>
          <span className="flex items-center gap-1">
            <ArrowUpFromLine
              className="h-3 w-3"
              style={{ color: 'var(--chart-legend-tx)' }}
              aria-hidden="true"
            />
            TX {formatRate(latestTx)}
          </span>
          <span className="sr-only">{mediumLabel} throughput</span>
        </div>
      </CardContent>
    </Card>
  );
}
