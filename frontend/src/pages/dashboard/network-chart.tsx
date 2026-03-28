import { useMemo } from 'react';
import { AreaChart, Area, XAxis, YAxis, ResponsiveContainer, Tooltip } from 'recharts';
import { ArrowDownToLine, ArrowUpFromLine } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { useWebSocket } from '@/hooks/use-websocket';
import { formatRate } from '@/lib/utils';
import { computeNetworkRates } from '@/pages/dashboard/network-chart-utils';

export function NetworkChart() {
  const { dataPoints, connected } = useWebSocket();

  const chartData = useMemo(() => computeNetworkRates(dataPoints), [dataPoints]);

  const latestRx = chartData.length > 0 ? chartData[chartData.length - 1].rx : 0;
  const latestTx = chartData.length > 0 ? chartData[chartData.length - 1].tx : 0;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Network Throughput</CardTitle>
        <div className="flex items-center gap-2">
          <span className={`h-2 w-2 rounded-full ${connected ? 'bg-green-500' : 'bg-gray-400'}`} />
        </div>
      </CardHeader>
      <CardContent>
        {chartData.length < 2 ? (
          <div className="flex h-[120px] items-center justify-center text-sm text-gray-500 dark:text-gray-400">
            Collecting data…
          </div>
        ) : (
          <ResponsiveContainer width="100%" height={120}>
            <AreaChart data={chartData} margin={{ top: 5, right: 5, bottom: 0, left: -20 }}>
              <defs>
                <linearGradient id="rxGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="txGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#f59e0b" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#f59e0b" stopOpacity={0} />
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
                formatter={(value: number) => formatRate(value)}
              />
              <Area
                type="monotone"
                dataKey="rx"
                stroke="#3b82f6"
                fill="url(#rxGrad)"
                strokeWidth={1.5}
                name="Download"
              />
              <Area
                type="monotone"
                dataKey="tx"
                stroke="#f59e0b"
                fill="url(#txGrad)"
                strokeWidth={1.5}
                name="Upload"
              />
            </AreaChart>
          </ResponsiveContainer>
        )}
        <div className="mt-2 flex justify-center gap-4 text-xs text-gray-500 dark:text-gray-400">
          <span className="flex items-center gap-1">
            <ArrowDownToLine className="h-3 w-3 text-blue-500" />
            RX {formatRate(latestRx)}
          </span>
          <span className="flex items-center gap-1">
            <ArrowUpFromLine className="h-3 w-3 text-amber-500" />
            TX {formatRate(latestTx)}
          </span>
        </div>
      </CardContent>
    </Card>
  );
}
