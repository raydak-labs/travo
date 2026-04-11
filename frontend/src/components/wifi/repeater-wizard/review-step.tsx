import { Wifi, Radio, Loader2, ArrowLeft } from 'lucide-react';
import type { WifiScanResult } from '@shared/index';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { DialogFooter } from '@/components/ui/dialog';
import { SecurityBadge } from '@/components/wifi/security-badge';
import type { RepeaterUpstreamConfig, RepeaterApFormConfig } from './types';

type ReviewStepProps = {
  upstream: RepeaterUpstreamConfig;
  selectedNetwork: WifiScanResult | null;
  apSummaryLine: string;
  effectiveAPEncryption: string;
  apConfig: RepeaterApFormConfig;
  allowApOnStaRadio: boolean;
  applyError: string | null;
  applying: boolean;
  onBack: () => void;
  onApply: () => void;
};

export function RepeaterWizardReviewStep({
  upstream,
  selectedNetwork,
  apSummaryLine,
  effectiveAPEncryption,
  apConfig,
  allowApOnStaRadio,
  applyError,
  applying,
  onBack,
  onApply,
}: ReviewStepProps) {
  return (
    <div className="space-y-4">
      <div className="space-y-3">
        <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Upstream Connection</h3>
        <div className="rounded-lg border p-3">
          <div className="flex items-center gap-2">
            <Wifi className="h-4 w-4 text-gray-500" />
            <span className="text-sm font-medium">{upstream.ssid}</span>
            {selectedNetwork && <SecurityBadge encryption={selectedNetwork.encryption} />}
          </div>
        </div>

        <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Access Point</h3>
        <div className="rounded-lg border p-3">
          <div className="flex items-center gap-2">
            <Radio className="h-4 w-4 text-gray-500" />
            <span className="text-sm font-medium">{apSummaryLine}</span>
            <Badge variant="outline">
              {effectiveAPEncryption === 'none' ? 'Open' : effectiveAPEncryption.toUpperCase()}
            </Badge>
          </div>
          {apConfig.sameAsUpstream && (
            <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
              Same credentials as upstream
            </p>
          )}
          {allowApOnStaRadio && (
            <p className="mt-1 text-xs text-amber-600 dark:text-amber-400">
              Uplink-radio AP allowed (less stable on dual-radio setups).
            </p>
          )}
        </div>
      </div>

      {applyError && (
        <p className="text-sm text-red-600 dark:text-red-400" role="alert">
          {applyError}
        </p>
      )}

      <DialogFooter>
        <Button variant="outline" onClick={onBack} disabled={applying}>
          <ArrowLeft className="mr-1.5 h-4 w-4" />
          Back
        </Button>
        <Button onClick={onApply} disabled={applying}>
          {applying ? (
            <>
              <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
              Applying...
            </>
          ) : (
            'Apply Configuration'
          )}
        </Button>
      </DialogFooter>
    </div>
  );
}
