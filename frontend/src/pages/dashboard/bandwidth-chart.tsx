import { AreaChart, Area, XAxis, YAxis, ResponsiveContainer, Tooltip } from 'recharts';
import { Activity } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { useWebSocket, type StatsDataPoint } from '@/hooks/use-websocket';

function formatTime(timestamp: number): string {
  const date = new Date(timestamp);
  return `${date.getMinutes().toString().padStart(2, '0')}:${date.getSeconds().toString().padStart(2, '0')}`;
}

export function BandwidthChart() {
  const { dataPoints, connected } = useWebSocket();

  const chartData = dataPoints.map((point: StatsDataPoint) => ({
    time: formatTime(point.timestamp),
    cpu: point.cpu,
    memory: point.memoryTotal > 0 ? (point.memoryUsed / point.memoryTotal) * 100 : 0,
  }));

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">System Usage</CardTitle>
        <div className="flex items-center gap-2">
          <span className={`h-2 w-2 rounded-full ${connected ? 'bg-green-500' : 'bg-gray-400'}`} />
          <Activity className="h-4 w-4 text-gray-500 dark:text-gray-400" />
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
                <linearGradient id="cpuGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="memGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#10b981" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
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
                domain={[0, 100]}
                tick={{ fontSize: 10, fill: 'var(--chart-axis)' }}
                stroke="var(--chart-grid)"
                tickLine={false}
                axisLine={false}
                tickFormatter={(v: number) => `${v}%`}
              />
              <Tooltip
                contentStyle={{
                  backgroundColor: 'var(--chart-tooltip-bg)',
                  border: '1px solid var(--chart-tooltip-border)',
                  borderRadius: '6px',
                  color: 'var(--chart-tooltip-text)',
                  fontSize: '12px',
                }}
                formatter={(value) => `${Number(value ?? 0).toFixed(1)}%`}
              />
              <Area
                type="monotone"
                dataKey="cpu"
                stroke="#3b82f6"
                fill="url(#cpuGrad)"
                strokeWidth={1.5}
                name="CPU"
              />
              <Area
                type="monotone"
                dataKey="memory"
                stroke="#10b981"
                fill="url(#memGrad)"
                strokeWidth={1.5}
                name="Memory"
              />
            </AreaChart>
          </ResponsiveContainer>
        )}
        <div className="mt-2 flex justify-center gap-4 text-xs text-gray-500 dark:text-gray-400">
          <span className="flex items-center gap-1">
            <span className="inline-block h-2 w-2 rounded-full bg-blue-500" />
            CPU
          </span>
          <span className="flex items-center gap-1">
            <span className="inline-block h-2 w-2 rounded-full bg-emerald-500" />
            Memory
          </span>
        </div>
      </CardContent>
    </Card>
  );
}
