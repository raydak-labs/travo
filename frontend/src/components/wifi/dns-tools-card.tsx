import { RefreshCw, Shield, ShieldOff } from 'lucide-react';
import { toast } from 'sonner';
import {
  useCaptiveDNSBypass,
  useCaptiveDNSRestore,
  useCaptivePortal,
  useDNSMode,
} from '@/hooks/use-captive-portal';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { CardInset } from '@/components/ui/card-inset';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

export function DNSToolsCard() {
  const { data: status, refetch, isFetching } = useCaptivePortal();
  const { data: dnsMode, isLoading: dnsModeLoading } = useDNSMode();
  const dnsBypass = useCaptiveDNSBypass();
  const dnsRestore = useCaptiveDNSRestore();

  const dnsBypassed = status?.dns_bypassed ?? false;

  const handleDNSBypass = () => {
    dnsBypass.mutate(undefined, {
      onSuccess: () => {
        toast.success('DNS bypass active — using upstream DNS');
        void refetch();
      },
      onError: (e) => toast.error(e instanceof Error ? e.message : 'DNS bypass failed'),
    });
  };

  const handleDNSRestore = () => {
    dnsRestore.mutate(undefined, {
      onSuccess: () => {
        toast.success('DNS restored to original configuration');
        void refetch();
      },
      onError: (e) => toast.error(e instanceof Error ? e.message : 'DNS restore failed'),
    });
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle>DNS Configuration</CardTitle>
        <Button
          size="sm"
          variant="ghost"
          className="h-7 w-7 p-0"
          disabled={isFetching}
          onClick={() => void refetch()}
          aria-label="Refresh DNS status"
        >
          <RefreshCw className={`h-3.5 w-3.5 ${isFetching ? 'animate-spin' : ''}`} />
        </Button>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* DNS Mode display */}
        {dnsModeLoading ? (
          <Skeleton className="h-5 w-2/3" />
        ) : dnsMode ? (
          <div className="flex items-center gap-2">
            <Badge variant={dnsMode.mode === 'default' ? 'outline' : 'secondary'}>
              {dnsMode.mode === 'default' && 'Default DNS'}
              {dnsMode.mode === 'adguard-forwarding' && 'AdGuard Forwarding'}
              {dnsMode.mode === 'adguard-direct' && 'AdGuard Direct'}
            </Badge>
            <span className="text-xs text-gray-500 dark:text-gray-400">{dnsMode.description}</span>
          </div>
        ) : null}

        {/* Bypass status + actions */}
        <CardInset>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              {dnsBypassed ? (
                <ShieldOff className="h-4 w-4 text-amber-500" />
              ) : (
                <Shield className="h-4 w-4 text-green-500" />
              )}
              <span className="text-sm font-medium">
                {dnsBypassed ? 'DNS Bypass Active' : 'DNS Normal'}
              </span>
            </div>
            <Badge variant={dnsBypassed ? 'warning' : 'success'}>
              {dnsBypassed ? 'Bypassed' : 'Normal'}
            </Badge>
          </div>
          <p className="mt-2 text-xs text-gray-500 dark:text-gray-400">
            {dnsBypassed
              ? 'DNS is temporarily using upstream DHCP DNS. Restore when done with captive portal login.'
              : 'DNS is operating normally. Use bypass when a hotel/airport network requires a login page.'}
          </p>
          <div className="mt-3 flex gap-2">
            {!dnsBypassed ? (
              <Button
                size="sm"
                variant="outline"
                disabled={dnsBypass.isPending}
                onClick={handleDNSBypass}
              >
                {dnsBypass.isPending ? 'Bypassing…' : 'Bypass DNS'}
              </Button>
            ) : (
              <Button
                size="sm"
                variant="outline"
                disabled={dnsRestore.isPending}
                onClick={handleDNSRestore}
              >
                {dnsRestore.isPending ? 'Restoring…' : 'Restore DNS'}
              </Button>
            )}
          </div>
        </CardInset>
      </CardContent>
    </Card>
  );
}
