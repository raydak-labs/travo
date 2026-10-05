import { CardInset } from '@/components/ui/card-inset';
import { Gauge } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { InlineError } from '@/components/ui/inline-error';
import { StatValue } from '@/components/ui/stat-value';
import { useRunSpeedTest } from '@/hooks/use-system';

export function SpeedTestCard() {
  const speedTest = useRunSpeedTest();

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle>Speed Test</CardTitle>
        <Gauge className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-xs text-gray-500 dark:text-gray-400">
          Measures download speed from the router using a 10 MB test file from Cloudflare.
        </p>

        <Button size="sm" onClick={() => speedTest.mutate()} disabled={speedTest.isPending}>
          {speedTest.isPending ? 'Running…' : 'Run Speed Test'}
        </Button>

        {speedTest.isPending && (
          <div className="space-y-2">
            <Skeleton className="h-4 w-1/2" />
            <Skeleton className="h-4 w-1/3" />
            <Skeleton className="h-4 w-2/5" />
          </div>
        )}

        {speedTest.data && (
          <CardInset variant="muted">
            <div className="grid grid-cols-2 gap-2">
              <StatValue label="Download" value={`${speedTest.data.download_mbps} Mbps`} />
              <StatValue label="Latency" value={`${speedTest.data.ping_ms} ms`} />
              <StatValue label="Server" value={speedTest.data.server} />
            </div>
          </CardInset>
        )}

        {speedTest.isError && <InlineError>{speedTest.error.message}</InlineError>}
      </CardContent>
    </Card>
  );
}
