import { Cable } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { useUptimeLog } from '@/hooks/use-network';

export function UptimeLogCard() {
  const { data: uptimeLog } = useUptimeLog();

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Connection Uptime Log</CardTitle>
        <Cable className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent>
        {!uptimeLog || uptimeLog.length === 0 ? (
          <p className="text-sm text-gray-500">No connectivity events recorded yet.</p>
        ) : (
          <ol className="space-y-2">
            {uptimeLog.map((event, i) => {
              const isConnected = event.state === 'connected';
              const next = uptimeLog[i + 1];
              const durationMs = next ? event.timestamp - next.timestamp : null;
              const fmt = (ms: number) => {
                const s = Math.floor(ms / 1000);
                if (s < 60) return `${s}s`;
                const m = Math.floor(s / 60);
                if (m < 60) return `${m}m`;
                const h = Math.floor(m / 60);
                const rem = m % 60;
                return rem > 0 ? `${h}h ${rem}m` : `${h}h`;
              };
              return (
                <li key={event.timestamp} className="flex items-start gap-3 text-sm">
                  <span
                    className={`mt-1 h-2.5 w-2.5 shrink-0 rounded-full ${
                      isConnected ? 'bg-green-500' : 'bg-red-500'
                    }`}
                  />
                  <div className="flex flex-col">
                    <span className={isConnected ? 'text-green-700' : 'text-red-600'}>
                      {isConnected ? 'Connected' : 'Disconnected'}
                    </span>
                    <span className="text-xs text-gray-500">
                      {new Date(event.timestamp).toLocaleString()}
                      {durationMs !== null && durationMs > 0 && (
                        <> &mdash; lasted {fmt(durationMs)}</>
                      )}
                    </span>
                  </div>
                </li>
              );
            })}
          </ol>
        )}
      </CardContent>
    </Card>
  );
}
