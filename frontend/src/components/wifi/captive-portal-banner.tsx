import { AlertTriangle, ExternalLink, X } from 'lucide-react';
import { useCaptivePortal } from '@/hooks/use-captive-portal';
import { Button } from '@/components/ui/button';
import { useState } from 'react';

export function CaptivePortalBanner() {
  const { data: status } = useCaptivePortal();
  const [dismissed, setDismissed] = useState(false);

  if (!status?.detected || status?.can_reach_internet || dismissed) {
    return null;
  }

  return (
    <div
      role="alert"
      className="flex items-center justify-between gap-3 rounded-lg border border-yellow-300 bg-yellow-50 p-4 dark:border-yellow-700 dark:bg-yellow-950"
    >
      <div className="flex items-center gap-3">
        <AlertTriangle className="h-5 w-5 shrink-0 text-yellow-600 dark:text-yellow-400" />
        <p className="text-sm font-medium text-yellow-800 dark:text-yellow-200">
          Login required to access internet
        </p>
      </div>
      <div className="flex items-center gap-2">
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
        <Button size="sm" variant="ghost" onClick={() => setDismissed(true)} aria-label="Dismiss">
          <X className="h-4 w-4" />
        </Button>
      </div>
    </div>
  );
}
