import { Gauge } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useRunWireGuardSpeedTest } from '@/hooks/use-vpn';

export function VpnSpeedTestCard() {
  const speedTest = useRunWireGuardSpeedTest();

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">VPN Speed Test</CardTitle>
        <Gauge className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-sm text-gray-500 dark:text-gray-400">
          Measures download throughput and latency from the router through the WireGuard tunnel (traffic is bound to{' '}
          <span className="font-mono text-xs">wg0</span>). Enable WireGuard and wait for a handshake before running.
        </p>

        <Button size="sm" onClick={() => speedTest.mutate()} disabled={speedTest.isPending}>
          {speedTest.isPending ? 'Running…' : 'Run VPN Speed Test'}
        </Button>

        {speedTest.isPending && (
          <div className="space-y-2">
            <Skeleton className="h-4 w-1/2" />
            <Skeleton className="h-4 w-1/3" />
          </div>
        )}

        {speedTest.data && (
          <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
            <div className="grid grid-cols-2 gap-2">
              <span className="text-gray-500 dark:text-gray-400">Download</span>
              <span className="font-medium">{speedTest.data.download_mbps.toFixed(2)} Mbps</span>
              <span className="text-gray-500 dark:text-gray-400">Latency</span>
              <span className="font-medium">
                {speedTest.data.ping_ms > 0 ? `${speedTest.data.ping_ms.toFixed(1)} ms` : '—'}
              </span>
              <span className="text-gray-500 dark:text-gray-400">Server</span>
              <span>{speedTest.data.server}</span>
            </div>
          </div>
        )}

        {speedTest.isError && (
          <div className="rounded-md border border-red-200 bg-red-50 p-3 text-xs text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300">
            {speedTest.error.message}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
