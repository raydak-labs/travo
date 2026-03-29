import { useMemo } from 'react';
import { Activity } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { useWebSocket } from '@/hooks/use-websocket';
import { sortInterfaceNames } from './interface-traffic-utils';
import { InterfaceTrafficChartCard } from './interface-traffic-chart-card';

export function InterfaceTrafficCharts() {
  const { interfaceDataPoints, connected } = useWebSocket();

  const sortedNames = useMemo(
    () => sortInterfaceNames(Object.keys(interfaceDataPoints)),
    [interfaceDataPoints],
  );

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
              <InterfaceTrafficChartCard
                key={name}
                name={name}
                points={interfaceDataPoints[name]}
              />
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
