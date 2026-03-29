import { useMemo } from 'react';
import { AreaChart, Area, XAxis, YAxis, ResponsiveContainer, Tooltip } from 'recharts';
import { ArrowDownToLine, ArrowUpFromLine } from 'lucide-react';
import type { InterfaceDataPoint } from '@/hooks/use-websocket';
import { formatRate } from '@/lib/utils';
import { computeTrafficRates, interfaceTrafficLabel } from './interface-traffic-utils';

type InterfaceTrafficChartCardProps = {
  name: string;
  points: InterfaceDataPoint[];
};

export function InterfaceTrafficChartCard({ name, points }: InterfaceTrafficChartCardProps) {
  const chartData = useMemo(() => computeTrafficRates(points), [points]);

  const latestRx = chartData.length > 0 ? chartData[chartData.length - 1].rx : 0;
  const latestTx = chartData.length > 0 ? chartData[chartData.length - 1].tx : 0;

  const rxGradId = `rx-${name}`;
  const txGradId = `tx-${name}`;

  return (
    <div className="rounded-md border bg-white p-3 dark:border-white/10 dark:bg-gray-950">
      <div className="mb-2 flex items-center justify-between">
        <span className="text-sm font-medium text-gray-900 dark:text-white">
          {interfaceTrafficLabel(name)}
        </span>
        <span className="text-xs font-mono text-gray-400">{name}</span>
      </div>
      {chartData.length < 2 ? (
        <div className="flex h-[100px] items-center justify-center text-xs text-gray-400">
          Collecting data…
        </div>
      ) : (
        <ResponsiveContainer width="100%" height={100}>
          <AreaChart data={chartData} margin={{ top: 2, right: 2, bottom: 0, left: -25 }}>
            <defs>
              <linearGradient id={rxGradId} x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.3} />
                <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
              </linearGradient>
              <linearGradient id={txGradId} x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#f59e0b" stopOpacity={0.3} />
                <stop offset="95%" stopColor="#f59e0b" stopOpacity={0} />
              </linearGradient>
            </defs>
            <XAxis
              dataKey="time"
              tick={{ fontSize: 9 }}
              stroke="#9ca3af"
              tickLine={false}
              axisLine={false}
            />
            <YAxis
              tick={{ fontSize: 9 }}
              stroke="#9ca3af"
              tickLine={false}
              axisLine={false}
              tickFormatter={(v: number) => formatRate(v)}
            />
            <Tooltip
              contentStyle={{
                backgroundColor: 'rgba(0,0,0,0.8)',
                border: 'none',
                borderRadius: '6px',
                color: '#fff',
                fontSize: '11px',
              }}
              formatter={(value) => formatRate(Number(value ?? 0))}
            />
            <Area
              type="monotone"
              dataKey="rx"
              stroke="#3b82f6"
              fill={`url(#${rxGradId})`}
              strokeWidth={1.5}
              name="Download"
            />
            <Area
              type="monotone"
              dataKey="tx"
              stroke="#f59e0b"
              fill={`url(#${txGradId})`}
              strokeWidth={1.5}
              name="Upload"
            />
          </AreaChart>
        </ResponsiveContainer>
      )}
      <div className="mt-1 flex justify-center gap-3 text-xs text-gray-500 dark:text-gray-400">
        <span className="flex items-center gap-1">
          <ArrowDownToLine className="h-3 w-3 text-blue-500" />
          {formatRate(latestRx)}
        </span>
        <span className="flex items-center gap-1">
          <ArrowUpFromLine className="h-3 w-3 text-amber-500" />
          {formatRate(latestTx)}
        </span>
      </div>
    </div>
  );
}
