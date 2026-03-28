import { Power } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useNetworkStatus, useSetInterfaceState } from '@/hooks/use-network';

export function InterfacesCard() {
  const { data: network, isLoading } = useNetworkStatus();
  const setInterfaceState = useSetInterfaceState();

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Network Interfaces</CardTitle>
        <Power className="h-4 w-4 text-gray-500" />
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-8 w-full" />
            <Skeleton className="h-8 w-full" />
          </div>
        ) : network?.interfaces && network.interfaces.length > 0 ? (
          <div className="space-y-3">
            {network.interfaces.map((iface) => (
              <div
                key={iface.name}
                className="flex items-center justify-between rounded-md bg-gray-50 p-3 dark:bg-gray-900"
              >
                <div className="flex items-center gap-3">
                  <Badge variant={iface.is_up ? 'success' : 'secondary'}>
                    {iface.is_up ? 'Up' : 'Down'}
                  </Badge>
                  <div>
                    <span className="text-sm font-medium text-gray-900 dark:text-white">
                      {iface.name.toUpperCase()}
                    </span>
                    {iface.ip_address && (
                      <span className="ml-2 text-xs text-gray-500">{iface.ip_address}</span>
                    )}
                  </div>
                </div>
                <Button
                  variant={iface.is_up ? 'destructive' : 'default'}
                  size="sm"
                  onClick={() => setInterfaceState.mutate({ name: iface.name, up: !iface.is_up })}
                  disabled={setInterfaceState.isPending}
                >
                  {iface.is_up ? 'Bring Down' : 'Bring Up'}
                </Button>
              </div>
            ))}
          </div>
        ) : (
          <EmptyState message="No interfaces found" />
        )}
      </CardContent>
    </Card>
  );
}
