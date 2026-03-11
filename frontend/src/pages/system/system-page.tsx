import { useState } from 'react';
import { Server, Cpu, HardDrive, Clock } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';
import { useSystemInfo, useSystemStats, useReboot } from '@/hooks/use-system';
import { formatBytes, formatUptime } from '@/lib/utils';

export function SystemPage() {
  const { data: info, isLoading: infoLoading } = useSystemInfo();
  const { data: stats, isLoading: statsLoading } = useSystemStats();
  const rebootMutation = useReboot();
  const [showRebootConfirm, setShowRebootConfirm] = useState(false);

  return (
    <div className="space-y-6">
      {/* System Info */}
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
                <span className="text-gray-900 dark:text-white">{info.hostname}</span>
                <span className="text-gray-500">Model</span>
                <span className="text-gray-900 dark:text-white">{info.model}</span>
                <span className="text-gray-500">Firmware</span>
                <span className="text-gray-900 dark:text-white">{info.firmware_version}</span>
                <span className="text-gray-500">Kernel</span>
                <span className="text-gray-900 dark:text-white">{info.kernel_version}</span>
              </div>
            </div>
          ) : null}
        </CardContent>
      </Card>

      {/* Uptime */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Uptime</CardTitle>
          <Clock className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          {infoLoading ? (
            <Skeleton className="h-4 w-1/2" />
          ) : info ? (
            <p className="text-lg font-medium text-gray-900 dark:text-white">
              {formatUptime(info.uptime_seconds)}
            </p>
          ) : null}
        </CardContent>
      </Card>

      {/* System Stats */}
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
              {/* CPU */}
              <div>
                <div className="mb-1 flex items-center justify-between text-sm">
                  <span className="text-gray-700 dark:text-gray-300">CPU</span>
                  <span className="text-gray-900 dark:text-white">
                    {stats.cpu.usage_percent.toFixed(1)}%
                    {stats.cpu.temperature_celsius != null && (
                      <span className="ml-2 text-gray-500">{stats.cpu.temperature_celsius}°C</span>
                    )}
                  </span>
                </div>
                <Progress value={stats.cpu.usage_percent} />
                <p className="mt-0.5 text-xs text-gray-500">
                  Load: {stats.cpu.load_average.map((v) => v.toFixed(2)).join(', ')} ·{' '}
                  {stats.cpu.cores} cores
                </p>
              </div>

              {/* Memory */}
              <div>
                <div className="mb-1 flex items-center justify-between text-sm">
                  <span className="text-gray-700 dark:text-gray-300">Memory</span>
                  <span className="text-gray-900 dark:text-white">
                    {stats.memory.usage_percent.toFixed(1)}% ({formatBytes(stats.memory.used_bytes)}{' '}
                    / {formatBytes(stats.memory.total_bytes)})
                  </span>
                </div>
                <Progress value={stats.memory.usage_percent} />
              </div>

              {/* Storage */}
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

      {/* Actions */}
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">Actions</CardTitle>
        </CardHeader>
        <CardContent>
          {showRebootConfirm ? (
            <div className="flex items-center gap-3">
              <Badge variant="warning">Confirm reboot?</Badge>
              <Button
                size="sm"
                variant="destructive"
                onClick={() => {
                  rebootMutation.mutate();
                  setShowRebootConfirm(false);
                }}
                disabled={rebootMutation.isPending}
              >
                {rebootMutation.isPending ? 'Rebooting…' : 'Reboot Now'}
              </Button>
              <Button size="sm" variant="outline" onClick={() => setShowRebootConfirm(false)}>
                Cancel
              </Button>
            </div>
          ) : (
            <Button size="sm" variant="destructive" onClick={() => setShowRebootConfirm(true)}>
              Reboot
            </Button>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
