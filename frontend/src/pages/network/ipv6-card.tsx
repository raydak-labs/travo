import { Globe } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { useIPv6Status, useSetIPv6Enabled } from '@/hooks/use-network';

export function IPv6Card() {
  const { data: status, isLoading } = useIPv6Status();
  const setIPv6Enabled = useSetIPv6Enabled();

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">IPv6</CardTitle>
        <Globe className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent className="space-y-4">
        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-6 w-full" />
            <Skeleton className="h-4 w-3/4" />
          </div>
        ) : (
          <>
            <div className="flex items-center justify-between">
              <span className="text-sm text-gray-700 dark:text-gray-300">Enable IPv6</span>
              <Switch
                checked={status?.enabled ?? false}
                onChange={(e) => setIPv6Enabled.mutate(e.target.checked)}
                disabled={setIPv6Enabled.isPending}
              />
            </div>
            <div>
              <p className="mb-1 text-xs font-medium text-gray-500">Global IPv6 Addresses</p>
              {status?.addresses && status.addresses.length > 0 ? (
                <ul className="space-y-1">
                  {status.addresses.map((addr) => (
                    <li key={addr} className="font-mono text-xs text-gray-900 dark:text-white">
                      {addr}
                    </li>
                  ))}
                </ul>
              ) : (
                <p className="text-xs text-gray-500">No global IPv6 addresses</p>
              )}
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}
