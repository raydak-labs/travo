import { Activity } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';
import { formatBytes, formatUptime } from '@/lib/utils';
import { useSystemInfo, useSystemStats } from '@/hooks/use-system';

export function SystemStatsCard() {
  const { data: stats, isLoading: statsLoading } = useSystemStats();
  const { data: info, isLoading: infoLoading } = useSystemInfo();

  const isLoading = statsLoading || infoLoading;

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>System Stats</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-1/2" />
        </CardContent>
      </Card>
    );
  }

  if (!stats) {
    return (
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">System Stats</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-1/2" />
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">System Stats</CardTitle>
        <Activity className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent className="space-y-4">
        <div>
          <div className="mb-1 flex justify-between text-sm">
            <span className="text-gray-600 dark:text-gray-400">CPU</span>
            <span className="font-medium text-gray-900 dark:text-white">
              {stats.cpu.usage_percent.toFixed(1)}%
            </span>
          </div>
          <Progress value={stats.cpu.usage_percent} />
        </div>
        <div>
          <div className="mb-1 flex justify-between text-sm">
            <span className="text-gray-600 dark:text-gray-400">Memory</span>
            <span className="font-medium text-gray-900 dark:text-white">
              {formatBytes(stats.memory.used_bytes)} / {formatBytes(stats.memory.total_bytes)}
            </span>
          </div>
          <Progress value={stats.memory.usage_percent} />
        </div>
        <div className="text-sm text-gray-600 dark:text-gray-400">
          Uptime:{' '}
          <span className="font-medium text-gray-900 dark:text-white">
            {formatUptime(info?.uptime_seconds ?? 0)}
          </span>
        </div>
      </CardContent>
    </Card>
  );
}
