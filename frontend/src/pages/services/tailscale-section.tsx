import { Globe, ExternalLink } from 'lucide-react';
import { Link } from '@tanstack/react-router';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { StatusPill } from '@/components/ui/status-pill';
import { Switch } from '@/components/ui/switch';
import { QueryCard } from '@/components/ui/query-card';
import { Skeleton } from '@/components/ui/skeleton';
import { OperationProgressDialog } from '@/components/ui/operation-progress-dialog';
import { useServices } from '@/hooks/use-services';
import {
  useTailscaleStatus,
  useToggleTailscale,
  useSetTailscaleExitNode,
  useTailscaleSSH,
  useSetTailscaleSSH,
  useVpnStatus,
} from '@/hooks/use-vpn';
import { isSafeExternalUrl } from '@/lib/external-url';
import { TailscaleAuthSection } from './tailscale-auth-section';
import { TailscaleLoggedInPanel } from './tailscale-logged-in-panel';

export function TailscaleSection() {
  const statusQuery = useTailscaleStatus();
  const { data: status, isLoading } = statusQuery;
  const vpnStatusQuery = useVpnStatus();
  const servicesQuery = useServices();
  const toggleMutation = useToggleTailscale();
  const exitNodeMutation = useSetTailscaleExitNode();
  const { data: sshStatus } = useTailscaleSSH();
  const sshMutation = useSetTailscaleSSH();

  const tsService = servicesQuery.data?.find((s) => s.id === 'tailscale');
  // "Not installed" may only be concluded from a service list or VPN status that
  // actually arrived; a failed GET must not read as an absent feature.
  const isInstalled = tsService ? tsService.state !== 'not_installed' : !!status;
  const installStatePending =
    !(servicesQuery.isSuccess || servicesQuery.isError) ||
    !(vpnStatusQuery.isSuccess || vpnStatusQuery.isError);
  const installStateFailed = servicesQuery.isError || vpnStatusQuery.isError;
  const statusFailed = isInstalled && statusQuery.isError;
  const wireguardEnabled =
    vpnStatusQuery.data?.some((s) => s.type === 'wireguard' && s.enabled) ?? false;

  const retryInstallState = () => {
    void servicesQuery.refetch();
    void vpnStatusQuery.refetch();
    void statusQuery.refetch();
  };

  return (
    <>
      <OperationProgressDialog
        open={toggleMutation.isPending}
        title={toggleMutation.variables ? 'Enabling Tailscale…' : 'Disabling Tailscale…'}
        description="Starting or stopping the Tailscale service."
        details={['Updating service state', 'Refreshing status']}
      />
      <OperationProgressDialog
        open={exitNodeMutation.isPending}
        title="Applying exit node…"
        description="WireGuard may be turned off first, then Tailscale exit routing is updated."
        details={['Single-VPN policy', 'Tailscale configuration', 'Refreshing status']}
      />
      <Card className={!isInstalled ? 'opacity-60' : undefined}>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle>Tailscale</CardTitle>
          <Globe className="h-4 w-4 text-gray-500 dark:text-gray-400" />
        </CardHeader>
        <CardContent className="space-y-4">
          <QueryCard
            isLoading={installStatePending || (isInstalled && isLoading)}
            isError={installStateFailed || statusFailed}
            error={servicesQuery.error ?? vpnStatusQuery.error ?? statusQuery.error}
            onRetry={retryInstallState}
            loading={
              <div className="space-y-2">
                <Skeleton className="h-4 w-3/4" />
                <Skeleton className="h-4 w-1/2" />
              </div>
            }
          >
            {!isInstalled ? (
              <div className="py-4 text-center">
                <p className="mb-2 text-sm">Tailscale is not installed on this router</p>
                <Link
                  to="/services"
                  className="text-sm text-blue-600 hover:underline dark:text-blue-400"
                >
                  Install via Services →
                </Link>
              </div>
            ) : status ? (
              <>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="text-sm text-gray-700 dark:text-gray-300">Status</span>
                    <StatusPill tone={status.logged_in ? 'ok' : 'neutral'} withDot>
                      {status.logged_in ? 'Logged In' : 'Logged Out'}
                    </StatusPill>
                    {status.running && <StatusPill tone="ok">Running</StatusPill>}
                  </div>
                  <Switch
                    id="tailscale-toggle"
                    label="Enable"
                    checked={status.running}
                    onChange={() => toggleMutation.mutate(!status.running)}
                    disabled={toggleMutation.isPending}
                  />
                </div>

                {!status.logged_in &&
                  status.running &&
                  (isSafeExternalUrl(status.auth_url) ? (
                    <a
                      href={status.auth_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="flex items-center gap-1 text-sm text-blue-600 hover:underline dark:text-blue-400"
                    >
                      Complete login at Tailscale <ExternalLink className="h-3 w-3" />
                    </a>
                  ) : (
                    <TailscaleAuthSection />
                  ))}

                {status.logged_in && (
                  <TailscaleLoggedInPanel
                    status={status}
                    wireguardEnabled={wireguardEnabled}
                    sshEnabled={sshStatus?.enabled ?? false}
                    sshPending={sshMutation.isPending}
                    onSshToggle={() => sshMutation.mutate(!(sshStatus?.enabled ?? false))}
                    onClearExitNode={() => exitNodeMutation.mutate('')}
                    onSetExitNode={(ip) => exitNodeMutation.mutate(ip)}
                    exitNodePending={exitNodeMutation.isPending}
                  />
                )}
              </>
            ) : null}
          </QueryCard>
        </CardContent>
      </Card>
    </>
  );
}
