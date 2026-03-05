import { Globe } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import { Skeleton } from '@/components/ui/skeleton';
import { useTailscaleStatus, useToggleTailscale } from '@/hooks/use-vpn';

export function TailscaleSection() {
  const { data: status, isLoading } = useTailscaleStatus();
  const toggleMutation = useToggleTailscale();

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Tailscale</CardTitle>
        <Globe className="h-4 w-4 text-blue-500" />
      </CardHeader>
      <CardContent className="space-y-4">
        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="h-4 w-1/2" />
          </div>
        ) : status ? (
          <>
            {/* Status and Toggle */}
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className="text-sm text-gray-700 dark:text-gray-300">Status</span>
                <Badge variant={status.logged_in ? 'success' : 'outline'}>
                  {status.logged_in ? 'Logged In' : 'Logged Out'}
                </Badge>
                {status.running && <Badge variant="success">Running</Badge>}
              </div>
              <Switch
                id="tailscale-toggle"
                label="Enable"
                checked={status.running}
                onChange={() => toggleMutation.mutate(!status.running)}
                disabled={toggleMutation.isPending}
              />
            </div>

            {/* Details */}
            {status.logged_in ? (
              <div className="rounded-md bg-gray-50 p-3 text-sm dark:bg-gray-900">
                <div className="grid grid-cols-2 gap-2">
                  <span className="text-gray-500">IP Address</span>
                  <span className="text-gray-900 dark:text-white">{status.ip_address}</span>
                  <span className="text-gray-500">Hostname</span>
                  <span className="text-gray-900 dark:text-white">{status.hostname}</span>
                  {status.exit_node && (
                    <>
                      <span className="text-gray-500">Exit Node</span>
                      <span className="text-gray-900 dark:text-white">
                        {status.exit_node}
                        {status.exit_node_active ? ' (active)' : ' (inactive)'}
                      </span>
                    </>
                  )}
                </div>
              </div>
            ) : (
              <p className="text-sm text-gray-500">
                Not logged in. Run{' '}
                <code className="rounded bg-gray-100 px-1 dark:bg-gray-800">tailscale login</code>{' '}
                on the device to authenticate.
              </p>
            )}
          </>
        ) : (
          <p className="text-sm text-gray-500">Tailscale is not installed</p>
        )}
      </CardContent>
    </Card>
  );
}
