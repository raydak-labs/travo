import { ArrowDownToLine, ArrowUpFromLine, HardDrive } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { formatBytes } from '@/lib/utils';
import { useSystemStats } from '@/hooks/use-system';

export function DataUsageCard() {
  const { data: stats, isLoading } = useSystemStats();

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Data Usage</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-3/4" />
          <Skeleton className="h-4 w-1/2" />
        </CardContent>
      </Card>
    );
  }

  // Find WAN interface stats (first interface, typically the WAN)
  const wan = stats?.network?.[0];
  const rxBytes = wan?.rx_bytes ?? 0;
  const txBytes = wan?.tx_bytes ?? 0;
  const totalBytes = rxBytes + txBytes;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Data Usage</CardTitle>
        <HardDrive className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="text-2xl font-bold text-gray-900 dark:text-white">
          {formatBytes(totalBytes)}
        </div>
        <div className="space-y-1.5 text-sm">
          <div className="flex items-center justify-between">
            <span className="flex items-center gap-1.5 text-gray-600 dark:text-gray-400">
              <ArrowDownToLine className="h-3.5 w-3.5 text-blue-500" />
              Downloaded
            </span>
            <span className="font-medium text-gray-900 dark:text-white">
              {formatBytes(rxBytes)}
            </span>
          </div>
          <div className="flex items-center justify-between">
            <span className="flex items-center gap-1.5 text-gray-600 dark:text-gray-400">
              <ArrowUpFromLine className="h-3.5 w-3.5 text-amber-500" />
              Uploaded
            </span>
            <span className="font-medium text-gray-900 dark:text-white">
              {formatBytes(txBytes)}
            </span>
          </div>
        </div>
        <p className="text-xs text-gray-500 dark:text-gray-400">Since last boot</p>
      </CardContent>
    </Card>
  );
}
