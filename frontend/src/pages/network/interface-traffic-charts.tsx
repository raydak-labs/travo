import { useMemo } from 'react';
import { AreaChart, Area, XAxis, YAxis, ResponsiveContainer, Tooltip } from 'recharts';
import { ArrowDownToLine, ArrowUpFromLine, Activity } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { useWebSocket, type InterfaceDataPoint } from '@/hooks/use-websocket';
import { formatRate } from '@/lib/utils';

const INTERFACE_LABELS: Record<string, string> = {
  'br-lan': 'LAN',
  eth0: 'WAN (Ethernet)',
  wwan0: 'WWAN (WiFi)',
  wg0: 'WireGuard VPN',
};

function interfaceLabel(name: string): string {
  return INTERFACE_LABELS[name] ?? name;
}

function formatTime(timestamp: number): string {
  const date = new Date(timestamp);
  return `${date.getMinutes().toString().padStart(2, '0')}:${date.getSeconds().toString().padStart(2, '0')}`;
}

interface RatePoint {
  time: string;
  rx: number;
  tx: number;
}

function computeRates(points: InterfaceDataPoint[]): RatePoint[] {
  const rates: RatePoint[] = [];
  for (let i = 1; i < points.length; i++) {
    const prev = points[i - 1];
    const curr = points[i];
    const dtSec = (curr.timestamp - prev.timestamp) / 1000;
    if (dtSec <= 0) continue;

    const rxDiff = curr.rxBytes - prev.rxBytes;
    const txDiff = curr.txBytes - prev.txBytes;

    rates.push({
      time: formatTime(curr.timestamp),
      rx: rxDiff > 0 ? rxDiff / dtSec : 0,
      tx: txDiff > 0 ? txDiff / dtSec : 0,
    });
  }
  return rates;
}

function InterfaceChart({ name, points }: { name: string; points: InterfaceDataPoint[] }) {
  const chartData = useMemo(() => computeRates(points), [points]);

  const latestRx = chartData.length > 0 ? chartData[chartData.length - 1].rx : 0;
  const latestTx = chartData.length > 0 ? chartData[chartData.length - 1].tx : 0;

  // Unique gradient IDs per interface to avoid SVG conflicts
  const rxGradId = `rx-${name}`;
  const txGradId = `tx-${name}`;

  return (
    <div className="rounded-md border bg-white p-3 dark:border-gray-800 dark:bg-gray-950">
      <div className="mb-2 flex items-center justify-between">
        <span className="text-sm font-medium text-gray-900 dark:text-white">
          {interfaceLabel(name)}
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
              formatter={(value: number) => formatRate(value)}
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

// Stable ordering for display
const INTERFACE_ORDER = ['eth0', 'br-lan', 'wwan0', 'wg0'];

export function InterfaceTrafficCharts() {
  const { interfaceDataPoints, connected } = useWebSocket();

  const sortedNames = useMemo(() => {
    const names = Object.keys(interfaceDataPoints);
    return names.sort((a, b) => {
      const ai = INTERFACE_ORDER.indexOf(a);
      const bi = INTERFACE_ORDER.indexOf(b);
      return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi);
    });
  }, [interfaceDataPoints]);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Interface Traffic</CardTitle>
        <div className="flex items-center gap-2">
          <span className={`h-2 w-2 rounded-full ${connected ? 'bg-green-500' : 'bg-gray-400'}`} />
          <Activity className="h-4 w-4 text-gray-500" />
        </div>
      </CardHeader>
      <CardContent>
        {sortedNames.length === 0 ? (
          <div className="flex h-[100px] items-center justify-center text-sm text-gray-500 dark:text-gray-400">
            Waiting for interface data…
          </div>
        ) : (
          <div className="grid gap-3 sm:grid-cols-2">
            {sortedNames.map((name) => (
              <InterfaceChart key={name} name={name} points={interfaceDataPoints[name]} />
            ))}
          </div>
        )}
        <div className="mt-3 flex justify-center gap-4 text-xs text-gray-500 dark:text-gray-400">
          <span className="flex items-center gap-1">
            <span className="inline-block h-2 w-2 rounded-full bg-blue-500" />
            Download (RX)
          </span>
          <span className="flex items-center gap-1">
            <span className="inline-block h-2 w-2 rounded-full bg-amber-500" />
            Upload (TX)
          </span>
        </div>
      </CardContent>
    </Card>
  );
}
