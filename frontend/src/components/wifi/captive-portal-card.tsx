import { CheckCircle2, ExternalLink, Globe, RefreshCw, ShieldAlert, WifiOff } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { toast } from 'sonner';
import {
  useCaptiveAutoAccept,
  useCaptiveDNSBypass,
  useCaptiveDNSRestore,
  useCaptivePortal,
} from '@/hooks/use-captive-portal';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { clearPortalNotification, notifyPortalOnce } from '@/lib/captive-portal-notifier';
import { isSafeExternalUrl, openExternalUrl } from '@/lib/external-url';

const AUTO_TRY_KEY = 'openwrt-travel-gui:captive-auto-try';

export function CaptivePortalCard() {
  const { data: status, isLoading, refetch, isFetching } = useCaptivePortal();
  const autoAccept = useCaptiveAutoAccept();
  const dnsBypass = useCaptiveDNSBypass();
  const dnsRestore = useCaptiveDNSRestore();
  const [autoTry, setAutoTry] = useState(() => localStorage.getItem(AUTO_TRY_KEY) === '1');
  const [confirmingBypass, setConfirmingBypass] = useState(false);
  const triedForPortalRef = useRef<string | null>(null);

  // Clear notification tracking when portal clears
  useEffect(() => {
    if (!status?.detected && status?.portal_url) {
      clearPortalNotification(status.portal_url);
    }
    if (!status?.detected) {
      triedForPortalRef.current = null;
    }
  }, [status?.detected, status?.portal_url]);

  // Fire toast ONCE per portal URL (survives page navigation)
  useEffect(() => {
    if (!status?.detected || status.can_reach_internet || !status.sta_connected) return;
    const portalUrl = status.portal_url ?? 'unknown';
    notifyPortalOnce(portalUrl, () => {
      toast.warning('Captive portal detected — login required', {
        description: `Portal: ${portalUrl}`,
        duration: 10000,
        action:
          portalUrl !== 'unknown' && isSafeExternalUrl(portalUrl)
            ? { label: 'Open Login', onClick: () => openExternalUrl(portalUrl) }
            : undefined,
      });
    });
  }, [status]);

  // Auto-try logic (only when sta_connected)
  useEffect(() => {
    if (!status?.detected || status.can_reach_internet || !autoTry || !status.sta_connected) return;

    if (status.dns_bypass_needed && !dnsBypass.isPending) {
      dnsBypass.mutate(undefined, {
        onSuccess: () => toast.info('DNS auto-bypassed for portal access'),
      });
      return;
    }

    const portal = status.portal_url ?? '';
    if (!portal || triedForPortalRef.current === portal) return;
    triedForPortalRef.current = portal;

    autoAccept.mutate(
      { portal_url: portal },
      {
        onSuccess: (res) => {
          if (res.ok) toast.success(res.message ?? 'Internet should be available');
          else toast.message(res.message ?? 'Auto-accept did not clear the captive portal');
        },
        onError: (e) => toast.error(e instanceof Error ? e.message : 'Auto-accept failed'),
      },
    );
  }, [status, autoTry, autoAccept, dnsBypass]);

  const handleToggleAutoTry = (enabled: boolean) => {
    setAutoTry(enabled);
    localStorage.setItem(AUTO_TRY_KEY, enabled ? '1' : '0');
    triedForPortalRef.current = null;
  };

  const runManualAutoAccept = () => {
    autoAccept.mutate(status?.portal_url ? { portal_url: status.portal_url } : {}, {
      onSuccess: (res) => {
        if (res.ok) toast.success(res.message ?? 'Internet should be available');
        else toast.message(res.message ?? 'Auto-accept did not clear the captive portal');
      },
      onError: (e) => toast.error(e instanceof Error ? e.message : 'Auto-accept failed'),
    });
  };

  const confirmDNSBypass = () => {
    setConfirmingBypass(false);
    handleDNSBypass();
  };

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

  if (isLoading) {
    return (
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle>Internet Access</CardTitle>
          <Globe className="h-4 w-4 text-gray-500 dark:text-gray-400" />
        </CardHeader>
        <CardContent className="space-y-3">
          <Skeleton className="h-5 w-1/3" />
          <Skeleton className="h-4 w-3/4" />
        </CardContent>
      </Card>
    );
  }

  const internetOk = status?.can_reach_internet ?? false;
  const portalDetected = status?.detected ?? false;
  const dnsBypassed = status?.dns_bypassed ?? false;
  const dnsNeedsBypass = status?.dns_bypass_needed ?? false;
  const portalUrl = status?.portal_url;
  const staConnected = status?.sta_connected ?? false;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle>Internet Access</CardTitle>
        <Button
          size="sm"
          variant="ghost"
          className="h-7 w-7 p-0"
          disabled={isFetching}
          onClick={() => void refetch()}
          aria-label="Re-check internet access"
        >
          <RefreshCw className={`h-3.5 w-3.5 ${isFetching ? 'animate-spin' : ''}`} />
        </Button>
      </CardHeader>

      <CardContent className="space-y-4">
        {/* Status indicator */}
        {!staConnected && (
          <div className="flex items-center gap-2.5">
            <WifiOff className="h-5 w-5 shrink-0 text-gray-500 dark:text-gray-400" />
            <div>
              <p className="text-sm font-medium leading-none">No Upstream</p>
              <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                Not connected to any upstream network
              </p>
            </div>
            <Badge variant="outline" className="ml-auto">
              Disconnected
            </Badge>
          </div>
        )}

        {staConnected && internetOk && !dnsBypassed && (
          <div className="flex items-center gap-2.5">
            <CheckCircle2 className="h-5 w-5 shrink-0 text-[var(--status-ok-text)]" />
            <div>
              <p className="text-sm font-medium leading-none">Connected</p>
              <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                No captive portal detected
              </p>
            </div>
            <Badge variant="success" className="ml-auto">
              OK
            </Badge>
          </div>
        )}

        {staConnected && internetOk && dnsBypassed && (
          <div className="flex items-center gap-2.5">
            <CheckCircle2 className="h-5 w-5 shrink-0 text-[var(--status-ok-text)]" />
            <div>
              <p className="text-sm font-medium leading-none">Connected</p>
              <p className="mt-0.5 text-xs text-[var(--status-warn-text)]">
                DNS bypass active — restore when done
              </p>
            </div>
            <Badge variant="warning" className="ml-auto">
              DNS Bypassed
            </Badge>
          </div>
        )}

        {staConnected && portalDetected && !internetOk && (
          <div className="flex items-center gap-2.5">
            <ShieldAlert className="h-5 w-5 shrink-0 text-[var(--status-warn-text)]" />
            <div className="min-w-0">
              <p className="text-sm font-medium leading-none">Login Required</p>
              {portalUrl && (
                <p className="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400">
                  {portalUrl}
                </p>
              )}
            </div>
            <Badge variant="warning" className="ml-auto shrink-0">
              Portal
            </Badge>
          </div>
        )}

        {staConnected && !portalDetected && !internetOk && (
          <div className="flex items-center gap-2.5">
            <WifiOff className="h-5 w-5 shrink-0 text-[var(--status-danger-text)]" />
            <div>
              <p className="text-sm font-medium leading-none">No Internet</p>
              <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                Connected but cannot reach internet
              </p>
            </div>
            <Badge variant="destructive" className="ml-auto">
              Offline
            </Badge>
          </div>
        )}

        {/* Portal action box — ONLY when genuinely detected + upstream connected */}
        {staConnected && portalDetected && !internetOk && (
          <div className="space-y-3 rounded-lg border border-[var(--status-warn-border)] bg-[var(--status-warn-surface)] p-3">
            {dnsNeedsBypass && (
              <div className="flex items-start gap-2">
                <ShieldAlert className="mt-0.5 h-3.5 w-3.5 shrink-0 text-[var(--status-warn-text)]" />
                <p className="text-xs text-[var(--status-warn-text)]">
                  Custom DNS is blocking portal access. Bypass DNS first, then open the login page.
                </p>
              </div>
            )}
            <div className="flex flex-wrap gap-2">
              {!dnsBypassed ? (
                <Button
                  size="sm"
                  variant={dnsNeedsBypass ? 'default' : 'outline'}
                  disabled={dnsBypass.isPending}
                  onClick={() => setConfirmingBypass(true)}
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
              {isSafeExternalUrl(portalUrl) && (
                <Button size="sm" variant="secondary" onClick={() => openExternalUrl(portalUrl)}>
                  <ExternalLink className="h-3.5 w-3.5" />
                  Open Login
                </Button>
              )}
              <Button
                size="sm"
                variant="ghost"
                disabled={autoAccept.isPending}
                onClick={runManualAutoAccept}
              >
                {autoAccept.isPending ? 'Trying…' : 'Auto-accept'}
              </Button>
            </div>
            <div className="border-t border-[var(--status-warn-border)] pt-2.5">
              <Switch
                id="captive-auto-try-card"
                label="Auto-submit portal logins on any network"
                checked={autoTry}
                onChange={(e) => handleToggleAutoTry(e.target.checked)}
              />
              <p className="mt-1 text-xs text-[var(--status-warn-text)]">
                Travo submits the sign-in form for any captive portal it detects on any network,
                without asking. Remembered on this device until you turn it off.
              </p>
            </div>
          </div>
        )}

        {/* Restore reminder when internet works but bypass still active */}
        {staConnected && internetOk && dnsBypassed && (
          <Button
            size="sm"
            variant="outline"
            className="w-full"
            disabled={dnsRestore.isPending}
            onClick={handleDNSRestore}
          >
            {dnsRestore.isPending ? 'Restoring…' : 'Restore DNS (AdGuard / dnsmasq)'}
          </Button>
        )}
        {/* Bypassing DNS overrides the configured resolver (AdGuard / dnsmasq)
            and persists until restored, which the button alone did not say. */}
        <ConfirmDialog
          open={confirmingBypass}
          onOpenChange={setConfirmingBypass}
          title="Bypass your DNS settings?"
          description="Portal logins need the network's own resolver, so this overrides your configured DNS."
          warningText="AdGuard / custom DNS filtering stays off until you choose Restore DNS. This affects every device on the network."
          confirmLabel="Bypass DNS"
          isPending={dnsBypass.isPending}
          onConfirm={confirmDNSBypass}
        />
      </CardContent>
    </Card>
  );
}
