import { AlertTriangle, ExternalLink, RefreshCw, X } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { toast } from 'sonner';
import {
  useCaptiveAutoAccept,
  useCaptiveDNSBypass,
  useCaptiveDNSRestore,
  useCaptivePortal,
} from '@/hooks/use-captive-portal';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';

const AUTO_TRY_KEY = 'openwrt-travel-gui:captive-auto-try';

export function CaptivePortalBanner() {
  const { data: status, refetch, isFetching } = useCaptivePortal();
  const autoAccept = useCaptiveAutoAccept();
  const dnsBypass = useCaptiveDNSBypass();
  const dnsRestore = useCaptiveDNSRestore();
  const [dismissed, setDismissed] = useState(false);
  const [autoTry, setAutoTry] = useState(() => localStorage.getItem(AUTO_TRY_KEY) === '1');
  const triedForPortalRef = useRef<string | null>(null);

  useEffect(() => {
    if (!status?.detected) {
      triedForPortalRef.current = null;
    }
  }, [status?.detected]);

  useEffect(() => {
    if (!status?.detected || status.can_reach_internet || dismissed || !autoTry) {
      return;
    }

    // Auto-bypass DNS if needed before attempting auto-accept
    if (status.dns_bypass_needed && !dnsBypass.isPending) {
      dnsBypass.mutate(undefined, {
        onSuccess: () => toast.info('DNS auto-bypassed for portal access'),
      });
      return;
    }

    const portal = status.portal_url ?? '';
    if (!portal) {
      return;
    }
    if (triedForPortalRef.current === portal) {
      return;
    }
    triedForPortalRef.current = portal;

    autoAccept.mutate(
      { portal_url: portal },
      {
        onSuccess: (res) => {
          if (res.ok) {
            toast.success(res.message ?? 'Internet should be available');
          } else {
            toast.message(res.message ?? 'Auto-accept did not clear the captive portal');
          }
        },
        onError: (e) => {
          toast.error(e instanceof Error ? e.message : 'Auto-accept failed');
        },
      },
    );
  }, [status, dismissed, autoTry, autoAccept, dnsBypass]);

  const handleToggleAutoTry = (enabled: boolean) => {
    setAutoTry(enabled);
    localStorage.setItem(AUTO_TRY_KEY, enabled ? '1' : '0');
    triedForPortalRef.current = null;
  };

  const runManualAutoAccept = () => {
    autoAccept.mutate(status?.portal_url ? { portal_url: status.portal_url } : {}, {
      onSuccess: (res) => {
        if (res.ok) {
          toast.success(res.message ?? 'Internet should be available');
        } else {
          toast.message(res.message ?? 'Auto-accept did not clear the captive portal');
        }
      },
      onError: (e) => {
        toast.error(e instanceof Error ? e.message : 'Auto-accept failed');
      },
    });
  };

  if (!status?.detected || status?.can_reach_internet || dismissed) {
    // Even when the banner is hidden, render a small "Check for captive portal"
    // button so the user can manually trigger detection when the app misses it.
    if (status?.can_reach_internet) {
      return null; // internet is fine, no need for a check button
    }
    return (
      <div className="flex justify-end">
        <Button
          size="sm"
          variant="ghost"
          className="text-xs text-muted-foreground"
          disabled={isFetching}
          onClick={() => void refetch()}
        >
          <RefreshCw className={`mr-1.5 h-3 w-3 ${isFetching ? 'animate-spin' : ''}`} />
          Check for captive portal
        </Button>
      </div>
    );
  }

  const handleDNSBypass = () => {
    dnsBypass.mutate(undefined, {
      onSuccess: () => toast.success('DNS switched to upstream for portal access'),
      onError: (e) => toast.error(e instanceof Error ? e.message : 'DNS bypass failed'),
    });
  };

  const handleDNSRestore = () => {
    dnsRestore.mutate(undefined, {
      onSuccess: () => toast.success('DNS restored to original configuration'),
      onError: (e) => toast.error(e instanceof Error ? e.message : 'DNS restore failed'),
    });
  };

  return (
    <div
      role="alert"
      className="flex flex-col gap-3 rounded-lg border border-yellow-300 bg-yellow-50 p-4 dark:border-yellow-700 dark:bg-yellow-950"
    >
      <div className="flex items-center gap-3">
        <AlertTriangle className="h-5 w-5 shrink-0 text-yellow-600 dark:text-yellow-400" />
        <p className="text-sm font-medium text-yellow-800 dark:text-yellow-200">
          Login required to access internet
        </p>
      </div>
      {status.dns_bypass_needed && (
        <p className="text-xs text-yellow-700 dark:text-yellow-300">
          Custom DNS is blocking portal access. Bypass DNS to open the login page.
        </p>
      )}
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <Switch
          id="captive-auto-try"
          label="Auto-try accept links"
          checked={autoTry}
          onChange={(e) => handleToggleAutoTry(e.target.checked)}
        />
        <div className="flex flex-wrap items-center gap-2">
          {status.dns_bypass_needed && (
            <Button
              size="sm"
              variant="destructive"
              disabled={dnsBypass.isPending}
              onClick={handleDNSBypass}
            >
              Bypass DNS
            </Button>
          )}
          {status.dns_bypassed && (
            <Button
              size="sm"
              variant="outline"
              disabled={dnsRestore.isPending}
              onClick={handleDNSRestore}
            >
              Restore DNS
            </Button>
          )}
          <Button
            size="sm"
            variant="secondary"
            disabled={autoAccept.isPending}
            onClick={runManualAutoAccept}
          >
            Try auto-accept
          </Button>
          {status.portal_url && (
            <Button
              size="sm"
              variant="outline"
              onClick={() => window.open(status.portal_url, '_blank')}
            >
              <ExternalLink className="mr-1.5 h-3.5 w-3.5" />
              Open Login Page
            </Button>
          )}
          <Button
            size="sm"
            variant="ghost"
            disabled={isFetching}
            onClick={() => void refetch()}
            aria-label="Re-check captive portal"
          >
            <RefreshCw className={`h-4 w-4 ${isFetching ? 'animate-spin' : ''}`} />
          </Button>
          <Button size="sm" variant="ghost" onClick={() => setDismissed(true)} aria-label="Dismiss">
            <X className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}
