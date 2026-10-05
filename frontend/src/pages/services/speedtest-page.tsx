import { Gauge } from 'lucide-react';
import {
  useSpeedtestServiceStatus,
  useInstallSpeedtestCLI,
  useUninstallSpeedtestCLI,
  useRunSpeedtest,
} from '@/hooks/use-speedtest';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { CardInset } from '@/components/ui/card-inset';
import { QueryCard } from '@/components/ui/query-card';
import { Skeleton } from '@/components/ui/skeleton';
import { StatValue } from '@/components/ui/stat-value';
import { InlineError } from '@/components/ui/inline-error';
import { EmptyState } from '@/components/ui/empty-state';

export function SpeedtestPage() {
  const {
    data: status,
    isLoading,
    isError,
    error: statusError,
    refetch,
  } = useSpeedtestServiceStatus();
  const installMutation = useInstallSpeedtestCLI();
  const uninstallMutation = useUninstallSpeedtestCLI();
  const runMutation = useRunSpeedtest();

  const isPending =
    installMutation.isPending || uninstallMutation.isPending || runMutation.isPending;

  // QueryCard reports the bare failure; keep the page's own wording in front
  // of it so it still names what would not load.
  const statusFailure =
    statusError === null || statusError === undefined
      ? null
      : new Error(
          `Failed to load speedtest service: ${
            statusError instanceof Error ? statusError.message : String(statusError)
          }`,
        );

  const retryStatus = () => void refetch();

  const cardLoading = (
    <div className="space-y-2">
      <Skeleton className="h-4 w-3/4" />
      <Skeleton className="h-4 w-1/2" />
    </div>
  );

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle>speedtest CLI</CardTitle>
          <Gauge className="h-4 w-4 text-gray-500 dark:text-gray-400" />
        </CardHeader>
        <CardContent className="space-y-4">
          <QueryCard
            isLoading={isLoading}
            isError={isError}
            error={statusFailure}
            onRetry={retryStatus}
            loading={cardLoading}
          >
            {status ? (
              <>
                <CardInset variant="muted">
                  <div className="grid gap-3 sm:grid-cols-2">
                    <StatValue label="Installed" value={status.installed ? 'Yes' : 'No'} />
                    <StatValue label="Supported" value={status.supported ? 'Yes' : 'No'} />
                    <StatValue
                      label="Architecture"
                      value={<span className="font-mono">{status.architecture || 'unknown'}</span>}
                    />
                    <StatValue
                      label="Version"
                      value={<span className="font-mono">{status.version || 'N/A'}</span>}
                    />
                    <StatValue
                      label="Package"
                      value={<span className="font-mono">{status.package_name}</span>}
                    />
                    <StatValue
                      label="Size"
                      value={<span className="font-mono">~{status.storage_size_mb} MB</span>}
                    />
                  </div>
                </CardInset>

                {!status.supported && (
                  <div className="rounded-md border border-[var(--status-warn-border)] bg-[var(--status-warn-surface)] p-3 text-sm text-[var(--status-warn-text)]">
                    Your router architecture ({status.architecture || 'unknown'}) is not supported
                    by the speedtest CLI package.
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
              </>
            ) : (
              <EmptyState message="No speedtest status available." />
            )}
          </QueryCard>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle>Run Speed Test</CardTitle>
          <Gauge className="h-4 w-4 text-gray-500 dark:text-gray-400" />
        </CardHeader>
        <CardContent className="space-y-4">
          <QueryCard
            isLoading={isLoading}
            isError={isError}
            error={statusFailure}
            onRetry={retryStatus}
            loading={cardLoading}
          >
            {status ? (
              <>
                <p className="text-sm text-gray-500 dark:text-gray-400">
                  Run a speed test using the Ookla speedtest CLI. Requires the CLI to be installed
                  first.
                </p>

                <Button
                  size="sm"
                  onClick={() => runMutation.mutate()}
                  disabled={isPending || !status.installed}
                >
                  {runMutation.isPending ? 'Running...' : 'Run Speed Test'}
                </Button>

                {runMutation.data && (
                  <CardInset variant="muted">
                    <div className="grid gap-3 sm:grid-cols-2">
                      <StatValue
                        label="Download"
                        value={`${runMutation.data.download_mbps.toFixed(2)} Mbps`}
                      />
                      <StatValue
                        label="Upload"
                        value={`${runMutation.data.upload_mbps.toFixed(2)} Mbps`}
                      />
                      <StatValue label="Ping" value={`${runMutation.data.ping_ms.toFixed(1)} ms`} />
                      <StatValue label="Server" value={runMutation.data.server} />
                    </div>
                  </CardInset>
                )}

                {runMutation.isError && (
                  <InlineError>{runMutation.error?.message || 'Speed test failed'}</InlineError>
                )}
              </>
            ) : null}
          </QueryCard>
        </CardContent>
      </Card>
    </div>
  );
}
