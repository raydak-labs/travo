import { Lock } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { useDoHStatus, useSetDoHEnabled } from '@/hooks/use-network';

export function DoHCard() {
  const { data: cfg, isLoading } = useDoHStatus();
  const setDoH = useSetDoHEnabled();

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">DNS over HTTPS/TLS</CardTitle>
        <Lock className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-4 w-1/2" />
            <Skeleton className="h-4 w-1/3" />
          </div>
        ) : (
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <div className="space-y-1">
                <p className="text-sm text-gray-900 dark:text-white">
                  {cfg?.enabled ? 'Enabled' : 'Disabled'}
                </p>
                {cfg?.provider && (
                  <Badge variant="outline" className="text-xs capitalize">
                    {cfg.provider}
                  </Badge>
                )}
              </div>
              <Switch
                checked={cfg?.enabled ?? false}
                onChange={() =>
                  cfg && setDoH.mutate({ ...cfg, enabled: !cfg.enabled })
                }
                disabled={setDoH.isPending}
                aria-label="Toggle DNS over HTTPS"
              />
            </div>
            <p className="text-xs text-gray-500">
              Encrypts DNS queries to prevent eavesdropping and tampering.
              Requires <code className="rounded bg-gray-100 px-1 dark:bg-gray-800">https-dns-proxy</code> to be installed.
            </p>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
