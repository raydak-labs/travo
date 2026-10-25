import { useState } from 'react';
import { ExternalLink, RefreshCw } from 'lucide-react';
import { API_ROUTES } from '@shared/index';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { CardInset } from '@/components/ui/card-inset';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { QueryCard } from '@/components/ui/query-card';
import { Skeleton } from '@/components/ui/skeleton';
import { StatusPill } from '@/components/ui/status-pill';
import { StreamLogDialog } from '@/components/stream-log-dialog';
import { useCheckUpdates, useRefreshUpdates, useUpdates } from '@/hooks/use-updates';
import { luciUpgradeUrl } from '@/lib/updates';

/** Anchor the header's update indicator links to. */
export const UPDATES_ANCHOR = 'updates';

const UPGRADE_WARNING =
  'Upgraded services restart: WiFi clients and DNS may drop for a few seconds. ' +
  'Some upgrades (kernel modules, the WiFi driver) only take full effect after a reboot.';

/**
 * OpenWrt releases and package updates from the router's last check. A new
 * release is flashed in LuCI (Attended Sysupgrade keeps the packages);
 * packages are upgraded here, streaming the package manager's output.
 */
export function SystemUpdatesCard() {
  const updates = useUpdates();
  const check = useCheckUpdates();
  const refresh = useRefreshUpdates();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [upgrading, setUpgrading] = useState(false);

  const st = updates.data;
  const firmware = st?.firmware;
  const packages = st?.packages;
  const checked = Boolean(st?.checked_at);

  return (
    <Card id={UPDATES_ANCHOR} className="scroll-mt-4">
      <CardHeader className="flex flex-row items-center justify-between gap-3 space-y-0 pb-2">
        <CardTitle>Updates</CardTitle>
        <div className="flex items-center gap-3">
          {checked && st && (
            <span className="hidden text-xs text-gray-500 sm:inline dark:text-gray-400">
              Checked {new Date(st.checked_at).toLocaleString()}
            </span>
          )}
          <Button
            size="sm"
            variant="outline"
            onClick={() => check.mutate()}
            disabled={check.isPending || upgrading}
          >
            <RefreshCw className={check.isPending ? 'h-3.5 w-3.5 animate-spin' : 'h-3.5 w-3.5'} />
            {check.isPending ? 'Checking…' : 'Check now'}
          </Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-3">
        <QueryCard
          isLoading={updates.isLoading}
          isError={updates.isError}
          error={updates.error}
          onRetry={() => void updates.refetch()}
          loading={<Skeleton className="h-16 w-full" />}
        >
          {!checked ? (
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Not checked yet. The router checks a few minutes after it starts, then daily.
            </p>
          ) : (
            <>
              {st && st.errors.length > 0 && (
                <p className="text-sm text-[var(--status-warn-text)]">
                  Part of the last check failed: {st.errors.join('; ')}
                </p>
              )}

              <CardInset variant="muted" className="space-y-2">
                {firmware?.latest ? (
                  <>
                    <StatusPill tone="info">OpenWrt {firmware.latest} available</StatusPill>
                    <p className="text-sm">
                      This router runs OpenWrt {firmware.current}.{' '}
                      {firmware.kind === 'release'
                        ? 'This is a new release series; read its release notes before upgrading.'
                        : 'This is a bug-fix and security release of the series you run.'}{' '}
                      Flash it in LuCI
                      {firmware.attended_sysupgrade
                        ? ' with Attended Sysupgrade, which keeps your installed packages.'
                        : '.'}
                    </p>
                    <Button size="sm" variant="outline" asChild>
                      <a
                        href={luciUpgradeUrl(
                          window.location.hostname,
                          firmware.attended_sysupgrade,
                        )}
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        <ExternalLink className="h-3.5 w-3.5" />
                        Upgrade in LuCI
                      </a>
                    </Button>
                  </>
                ) : (
                  <p className="text-sm">
                    OpenWrt {firmware?.current} is the newest release for this router.
                  </p>
                )}
              </CardInset>

              {packages && packages.manager !== 'none' && (
                <CardInset variant="muted" className="space-y-2">
                  {packages.upgradable.length > 0 ? (
                    <>
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <StatusPill tone="info">
                          {packages.upgradable.length} package update
                          {packages.upgradable.length === 1 ? '' : 's'}
                        </StatusPill>
                        <Button size="sm" onClick={() => setConfirmOpen(true)} disabled={upgrading}>
                          Upgrade all
                        </Button>
                      </div>
                      <details className="text-sm">
                        <summary className="cursor-pointer text-gray-600 dark:text-gray-400">
                          Show packages
                        </summary>
                        <ul className="mt-2 max-h-64 space-y-1 overflow-auto font-mono text-xs">
                          {packages.upgradable.map((p) => (
                            <li key={p.name} className="flex flex-wrap gap-x-2">
                              <span className="font-medium">{p.name}</span>
                              <span className="text-gray-500 dark:text-gray-400">
                                {p.current} → {p.available}
                              </span>
                            </li>
                          ))}
                        </ul>
                      </details>
                    </>
                  ) : (
                    <p className="text-sm">All installed packages are up to date.</p>
                  )}
                </CardInset>
              )}
            </>
          )}
        </QueryCard>
      </CardContent>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={`Upgrade ${packages?.upgradable.length ?? 0} packages?`}
        description={`Runs ${packages?.manager ?? 'the package manager'} upgrade on the router.`}
        warningText={UPGRADE_WARNING}
        confirmLabel="Upgrade"
        onConfirm={() => {
          setConfirmOpen(false);
          setUpgrading(true);
        }}
      />
      {upgrading && (
        <StreamLogDialog
          open={true}
          onOpenChange={(open) => !open && setUpgrading(false)}
          route={API_ROUTES.system.upgradePackagesStream}
          title="Upgrading packages"
          onComplete={() => {
            setUpgrading(false);
            refresh();
          }}
        />
      )}
    </Card>
  );
}
