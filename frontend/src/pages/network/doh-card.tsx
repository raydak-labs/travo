import { Lock } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { QueryCard } from '@/components/ui/query-card';
import { Skeleton } from '@/components/ui/skeleton';
import { StatusPill } from '@/components/ui/status-pill';
import { Switch } from '@/components/ui/switch';
import { useDoHConfig, useSetDoHConfig } from '@/hooks/use-network';

export function DoHCard() {
  const { data: cfg, isLoading, isError, error, refetch } = useDoHConfig();
  const setDoH = useSetDoHConfig();

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle>DNS over HTTPS/TLS</CardTitle>
        <Lock className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent>
        <QueryCard
          isLoading={isLoading}
          isError={isError}
          error={error}
          onRetry={() => void refetch()}
          loading={
            <div className="space-y-2">
              <Skeleton className="h-4 w-1/2" />
              <Skeleton className="h-4 w-1/3" />
            </div>
          }
        >
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <div className="space-y-1">
                <StatusPill tone={cfg?.enabled ? 'ok' : 'neutral'} withDot>
                  {cfg?.enabled ? 'Enabled' : 'Disabled'}
                </StatusPill>
                {cfg?.provider && (
                  <Badge variant="outline" className="text-xs capitalize">
                    {cfg.provider}
                  </Badge>
                )}
              </div>
              <Switch
                checked={cfg?.enabled ?? false}
                onChange={() => cfg && setDoH.mutate({ ...cfg, enabled: !cfg.enabled })}
                disabled={setDoH.isPending}
                aria-label="Toggle DNS over HTTPS"
              />
            </div>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Encrypts DNS queries to prevent eavesdropping and tampering. Requires{' '}
              <code className="rounded bg-gray-100 px-1 dark:bg-gray-800">https-dns-proxy</code> to
              be installed.
            </p>
          </div>
        </QueryCard>
      </CardContent>
    </Card>
  );
}
