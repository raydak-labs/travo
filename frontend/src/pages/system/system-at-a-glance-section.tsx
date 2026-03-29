import { Server, Cpu, HardDrive } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';
import { HostnameInlineForm } from './hostname-inline-form';
import { useSystemInfo, useSystemStats } from '@/hooks/use-system';
import { formatBytes, formatUptime } from '@/lib/utils';

export function SystemAtAGlanceSection() {
  const { data: info, isLoading: infoLoading, refetch: refetchInfo } = useSystemInfo();
  const { data: stats, isLoading: statsLoading } = useSystemStats();

  return (
    <div>
      <h2 className="mb-3 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500">
        At a Glance
      </h2>
      <div className="space-y-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">System Information</CardTitle>
            <Server className="h-4 w-4 text-gray-500" />
          </CardHeader>
          <CardContent>
            {infoLoading ? (
              <div className="space-y-2">
                <Skeleton className="h-4 w-3/4" />
                <Skeleton className="h-4 w-1/2" />
              </div>
            ) : info ? (
              <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
                <div className="grid grid-cols-2 gap-2">
                  <span className="text-gray-500">Hostname</span>
                  <span className="flex items-center gap-1 text-gray-900 dark:text-white">
                    <HostnameInlineForm hostname={info.hostname} onUpdated={() => refetchInfo()} />
                  </span>
                  <span className="text-gray-500">Model</span>
                  <span className="text-gray-900 dark:text-white">{info.model}</span>
                  <span className="text-gray-500">Firmware</span>
                  <span className="text-gray-900 dark:text-white">{info.firmware_version}</span>
                  <span className="text-gray-500">Kernel</span>
                  <span className="text-gray-900 dark:text-white">{info.kernel_version}</span>
                  <span className="text-gray-500">Uptime</span>
                  <span className="text-gray-900 dark:text-white">
                    {formatUptime(info.uptime_seconds)}
                  </span>
                </div>
              </div>
            ) : null}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">System Stats</CardTitle>
            <Cpu className="h-4 w-4 text-gray-500" />
          </CardHeader>
          <CardContent className="space-y-4">
            {statsLoading ? (
              <div className="space-y-4">
                <Skeleton className="h-8 w-full" />
                <Skeleton className="h-8 w-full" />
                <Skeleton className="h-8 w-full" />
              </div>
            ) : stats ? (
              <>
                <div>
                  <div className="mb-1 flex items-center justify-between text-sm">
                    <span className="text-gray-700 dark:text-gray-300">CPU</span>
                    <span className="text-gray-900 dark:text-white">
                      {stats.cpu.usage_percent.toFixed(1)}%
                      {stats.cpu.temperature_celsius != null && (
                        <span className="ml-2 text-gray-500">
                          {stats.cpu.temperature_celsius}°C
                        </span>
                      )}
                    </span>
                  </div>
                  <Progress value={stats.cpu.usage_percent} />
                  <p className="mt-0.5 text-xs text-gray-500">
                    Load: {stats.cpu.load_average.map((v) => v.toFixed(2)).join(', ')} ·{' '}
                    {stats.cpu.cores} cores
                  </p>
                </div>

                <div>
                  <div className="mb-1 flex items-center justify-between text-sm">
                    <span className="text-gray-700 dark:text-gray-300">Memory</span>
                    <span className="text-gray-900 dark:text-white">
                      {stats.memory.usage_percent.toFixed(1)}% (
                      {formatBytes(stats.memory.used_bytes)} /{' '}
                      {formatBytes(stats.memory.total_bytes)})
                    </span>
                  </div>
                  <Progress value={stats.memory.usage_percent} />
                </div>

                <div>
                  <div className="mb-1 flex items-center justify-between text-sm">
                    <span className="text-gray-700 dark:text-gray-300">
                      <span className="inline-flex items-center gap-1">
                        <HardDrive className="h-3.5 w-3.5" />
                        Storage
                      </span>
                    </span>
                    <span className="text-gray-900 dark:text-white">
                      {stats.storage.usage_percent.toFixed(1)}% (
                      {formatBytes(stats.storage.used_bytes)} /{' '}
                      {formatBytes(stats.storage.total_bytes)})
                    </span>
                  </div>
                  <Progress value={stats.storage.usage_percent} />
                </div>
              </>
            ) : null}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
