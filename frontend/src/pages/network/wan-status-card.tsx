import { statusDotClass, statusDotIdleClass } from '@/lib/status-dot';
import { Info, Cable, CheckCircle, XCircle } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { QueryCard } from '@/components/ui/query-card';
import { StatusPill } from '@/components/ui/status-pill';
import { CardInset } from '@/components/ui/card-inset';
import { useNetworkStatus, useFailoverConfig } from '@/hooks/use-network';
import { resolveUplinks, type ResolvedUplink } from '@/lib/uplink';
import type { NetworkInterface } from '@shared/index';

function UplinkRow({ uplink }: { uplink: ResolvedUplink }) {
  return (
    <div className="flex items-center gap-3">
      <span
        aria-hidden="true"
        className={`inline-block h-2.5 w-2.5 shrink-0 rounded-full ${
          uplink.active ? statusDotClass(true) : statusDotIdleClass
        }`}
      />
      <span className="sr-only">{`${uplink.longLabel}: ${uplink.active ? 'Active' : 'Inactive'}`}</span>
      <div className="flex-1">
        <span className="text-sm font-medium text-gray-900 dark:text-white">{uplink.label}</span>
        <span className="ml-2 text-xs text-gray-500 dark:text-gray-400">
          {uplink.active
            ? (uplink.iface?.ip_address ?? '')
            : uplink.iface
              ? uplink.inactiveCopy
              : uplink.notConfiguredCopy}
        </span>
      </div>
      <StatusPill tone={uplink.active ? 'ok' : 'neutral'}>
        {uplink.active ? 'Active' : 'Inactive'}
      </StatusPill>
    </div>
  );
}

/**
 * Explains what the router will actually do, which depends on whether
 * automatic failover is switched on. The previous copy claimed traffic
 * "automatically fails over to WiFi" unconditionally, while the Advanced tab
 * showed mwan3 as disabled — the two pages contradicted each other.
 */
function UplinkExplanation({
  anyActive,
  failoverEnabled,
  failoverAvailable,
}: {
  anyActive: boolean;
  failoverEnabled: boolean;
  failoverAvailable: boolean;
}) {
  let message: string;
  if (!anyActive) {
    message =
      'No uplink is active. Connect an ethernet cable, join an upstream WiFi network, or plug in a tethered phone to get internet access.';
  } else if (failoverEnabled) {
    message =
      'Automatic failover is on. The router monitors each uplink and moves traffic to the highest-priority one that is online.';
  } else if (failoverAvailable) {
    message =
      'Both uplinks can be connected at the same time; the router uses the wired one. Turn on Connection Failover under Advanced to switch automatically.';
  } else {
    message =
      'Both uplinks can be connected at the same time; the router prefers the wired link, then WiFi, then USB. Automatic switching is unavailable.';
  }

  return (
    <div className="flex items-start gap-2 rounded-md border border-[var(--status-info-border)] bg-[var(--status-info-surface)] p-3 text-xs text-[var(--status-info-text)]">
      <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" />
      <span>{message}</span>
    </div>
  );
}

function WanInterplay({ interfaces }: { interfaces: readonly NetworkInterface[] }) {
  const { data: failover } = useFailoverConfig();
  const uplinks = resolveUplinks(interfaces);
  const anyActive = uplinks.some((u) => u.active);

  return (
    <div className="space-y-3">
      <CardInset variant="muted" className="space-y-3">
        {uplinks.map((uplink) => (
          <UplinkRow key={uplink.key} uplink={uplink} />
        ))}
      </CardInset>

      <UplinkExplanation
        anyActive={anyActive}
        failoverEnabled={failover?.enabled === true}
        failoverAvailable={failover?.available === true}
      />
    </div>
  );
}

export function WanStatusCard() {
  const { data: network, isLoading, isError, error, refetch } = useNetworkStatus();
  // `internet_reachable` is a WAN-carrier check, not a reachability probe: a
  // DHCP lease on a dead or captive-portaled uplink still reads true. Naming it
  // "Internet" is the one word a non-expert trusts most.
  const uplinkUp = network?.internet_reachable === true;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle>WAN Status</CardTitle>
        <Cable className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>
      <CardContent className="space-y-4">
        <QueryCard
          isLoading={isLoading}
          isError={isError}
          error={error}
          onRetry={() => void refetch()}
          loading={
            <div className="space-y-2">
              <Skeleton className="h-4 w-1/3" />
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          }
        >
          <div className="flex items-center gap-2">
            {uplinkUp ? (
              <>
                <CheckCircle className="h-4 w-4 text-[var(--status-ok-text)]" aria-hidden="true" />
                <span className="text-sm font-medium text-gray-900 dark:text-white">
                  Uplink Connected
                </span>
                <span className="sr-only">WAN link is up.</span>
                <StatusPill tone="ok" withDot>
                  Online
                </StatusPill>
              </>
            ) : (
              <>
                <XCircle className="h-4 w-4 text-[var(--status-danger-text)]" aria-hidden="true" />
                <span className="text-sm font-medium text-gray-900 dark:text-white">No Uplink</span>
                <span className="sr-only">WAN link is down.</span>
                <StatusPill tone="danger" withDot>
                  Offline
                </StatusPill>
              </>
            )}
          </div>

          <WanInterplay interfaces={network?.interfaces ?? []} />
        </QueryCard>
      </CardContent>
    </Card>
  );
}
