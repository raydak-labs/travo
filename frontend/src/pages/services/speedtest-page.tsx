import { Gauge, Download, Upload, Clock, Server, AlertTriangle } from 'lucide-react';
import {
  useSpeedtestServiceStatus,
  useInstallSpeedtestCLI,
  useUninstallSpeedtestCLI,
  useRunSpeedtest,
} from '@/hooks/use-speedtest';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

export function SpeedtestPage() {
  const { data: status, isLoading, error: statusError } = useSpeedtestServiceStatus();
  const installMutation = useInstallSpeedtestCLI();
  const uninstallMutation = useUninstallSpeedtestCLI();
  const runMutation = useRunSpeedtest();

  const isPending = installMutation.isPending || uninstallMutation.isPending || runMutation.isPending;

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-48 w-full" />
        <Skeleton className="h-48 w-full" />
      </div>
    );
  }

  if (statusError) {
    return (
      <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300">
        <AlertTriangle className="mr-2 inline h-4 w-4" />
        Failed to load speedtest service: {statusError.message}
      </div>
    );
  }

  if (!status) {
    return null;
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-base font-medium">speedtest CLI</CardTitle>
          <Gauge className="h-5 w-5 text-gray-500" />
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 rounded-md bg-gray-50 p-4 text-sm dark:bg-gray-900 md:grid-cols-2">
            <div>
              <span className="text-gray-500 dark:text-gray-400">Installed</span>
              <span className="ml-2 font-medium">{status.installed ? 'Yes' : 'No'}</span>
            </div>
            <div>
              <span className="text-gray-500 dark:text-gray-400">Supported</span>
              <span className="ml-2 font-medium">{status.supported ? 'Yes' : 'No'}</span>
            </div>
            <div>
              <span className="text-gray-500 dark:text-gray-400">Architecture</span>
              <span className="ml-2 font-mono text-xs">{status.architecture || 'unknown'}</span>
            </div>
            <div>
              <span className="text-gray-500 dark:text-gray-400">Version</span>
              <span className="ml-2 font-mono text-xs">{status.version || 'N/A'}</span>
            </div>
            <div>
              <span className="text-gray-500 dark:text-gray-400">Package</span>
              <span className="ml-2 font-mono text-xs">{status.package_name}</span>
            </div>
            <div>
              <span className="text-gray-500 dark:text-gray-400">Size</span>
              <span className="ml-2 font-mono text-xs">~{status.storage_size_mb} MB</span>
            </div>
          </div>

          {!status.supported && (
            <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-300">
              Your router architecture ({status.architecture || 'unknown'}) is not supported by the
              speedtest CLI package.
            </div>
          )}

          <div className="flex gap-2">
            {status.installed ? (
              <Button
                variant="outline"
                size="sm"
                onClick={() => uninstallMutation.mutate()}
                disabled={isPending}
              >
                {uninstallMutation.isPending ? 'Removing...' : 'Uninstall'}
              </Button>
            ) : (
              <Button
                size="sm"
                onClick={() => installMutation.mutate()}
                disabled={isPending || !status.supported}
              >
                {installMutation.isPending ? 'Installing...' : 'Install speedtest CLI'}
              </Button>
            )}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-base font-medium">Run Speed Test</CardTitle>
          <Gauge className="h-5 w-5 text-gray-500" />
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-sm text-gray-500 dark:text-gray-400">
            Run a speed test using the Ookla speedtest CLI. Requires the CLI to be installed first.
          </p>

          <Button
            size="sm"
            onClick={() => runMutation.mutate()}
            disabled={isPending || !status.installed}
          >
            {runMutation.isPending ? 'Running...' : 'Run Speed Test'}
          </Button>

          {runMutation.data && (
            <div className="grid gap-3 rounded-md bg-gray-50 p-4 text-sm dark:bg-gray-900 md:grid-cols-2">
              <div className="flex items-center gap-2">
                <Download className="h-4 w-4 text-gray-500" />
                <span className="text-gray-500 dark:text-gray-400">Download</span>
                <span className="ml-auto font-medium">{runMutation.data.download_mbps.toFixed(2)} Mbps</span>
              </div>
              <div className="flex items-center gap-2">
                <Upload className="h-4 w-4 text-gray-500" />
                <span className="text-gray-500 dark:text-gray-400">Upload</span>
                <span className="ml-auto font-medium">{runMutation.data.upload_mbps.toFixed(2)} Mbps</span>
              </div>
              <div className="flex items-center gap-2">
                <Clock className="h-4 w-4 text-gray-500" />
                <span className="text-gray-500 dark:text-gray-400">Ping</span>
                <span className="ml-auto font-medium">{runMutation.data.ping_ms.toFixed(1)} ms</span>
              </div>
              <div className="flex items-center gap-2">
                <Server className="h-4 w-4 text-gray-500" />
                <span className="text-gray-500 dark:text-gray-400">Server</span>
                <span className="ml-auto font-medium">{runMutation.data.server}</span>
              </div>
            </div>
          )}

          {runMutation.isError && (
            <div className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300">
              {runMutation.error?.message || 'Speed test failed'}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}