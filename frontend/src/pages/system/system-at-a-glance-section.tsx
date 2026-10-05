import { CardInset } from '@/components/ui/card-inset';
import { Server, Cpu, HardDrive } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';
import { QueryCard } from '@/components/ui/query-card';
import { SectionHeading } from '@/components/ui/section-heading';
import { StatValue } from '@/components/ui/stat-value';
import { HostnameInlineForm } from './hostname-inline-form';
import { useSystemInfo, useSystemStats } from '@/hooks/use-system';
import { formatBytes, formatUptime } from '@/lib/utils';

export function SystemAtAGlanceSection() {
  const {
    data: info,
    isLoading: infoLoading,
    isError: infoError,
    error: infoErrorDetail,
    refetch: refetchInfo,
  } = useSystemInfo();
  const {
    data: stats,
    isLoading: statsLoading,
    isError: statsError,
    error: statsErrorDetail,
    refetch: refetchStats,
  } = useSystemStats();

  return (
    <div>
      <SectionHeading>At a Glance</SectionHeading>
      <div className="space-y-4">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle>System Information</CardTitle>
            <Server className="h-4 w-4 text-gray-500 dark:text-gray-400" />
          </CardHeader>
          <CardContent>
            {/* Without this the card body rendered nothing at all on failure,
                which reads as an empty section rather than a broken request. */}
            <QueryCard
              isLoading={infoLoading}
              isError={infoError}
              error={infoErrorDetail}
              onRetry={() => void refetchInfo()}
              loading={
                <div className="space-y-2">
                  <Skeleton className="h-4 w-3/4" />
                  <Skeleton className="h-4 w-1/2" />
                </div>
              }
            >
              {info ? (
                <CardInset variant="muted">
                  <div className="grid gap-3 sm:grid-cols-2">
                    <StatValue
                      label="Hostname"
                      value={
                        <HostnameInlineForm
                          hostname={info.hostname}
                          onUpdated={() => refetchInfo()}
                        />
                      }
                    />
                    <StatValue label="Model" value={info.model} />
                    <StatValue label="Firmware" value={info.firmware_version} />
                    <StatValue label="Kernel" value={info.kernel_version} />
                    <StatValue label="Uptime" value={formatUptime(info.uptime_seconds)} />
                  </div>
                </CardInset>
              ) : null}
            </QueryCard>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle>System Stats</CardTitle>
            <Cpu className="h-4 w-4 text-gray-500 dark:text-gray-400" />
          </CardHeader>
          <CardContent className="space-y-4">
            <QueryCard
              isLoading={statsLoading}
              isError={statsError}
              error={statsErrorDetail}
              onRetry={() => void refetchStats()}
              loading={
                <div className="space-y-4">
                  <Skeleton className="h-8 w-full" />
                  <Skeleton className="h-8 w-full" />
                  <Skeleton className="h-8 w-full" />
                </div>
              }
            >
              {stats ? (
                <>
                  <div>
                    <div className="mb-1 flex items-center justify-between text-sm">
                      <span className="text-gray-700 dark:text-gray-300">CPU</span>
                      <span className="text-gray-900 dark:text-white">
                        {stats.cpu.usage_percent.toFixed(1)}%
                        {stats.cpu.temperature_celsius != null && (
                          <span className="ml-2 text-gray-500 dark:text-gray-400">
                            {stats.cpu.temperature_celsius}°C
                          </span>
                        )}
                      </span>
                    </div>
                    <Progress value={stats.cpu.usage_percent} />
                    <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
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
            </QueryCard>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
