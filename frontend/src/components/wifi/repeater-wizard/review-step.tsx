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
  failedStep: string | null;
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
  failedStep,
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
            <Wifi className="h-4 w-4 text-gray-500 dark:text-gray-400" />
            <span className="text-sm font-medium">{upstream.ssid}</span>
            {selectedNetwork && <SecurityBadge encryption={selectedNetwork.encryption} />}
          </div>
        </div>

        <h3 className="text-sm font-semibold text-gray-900 dark:text-white">Access Point</h3>
        <div className="rounded-lg border p-3">
          <div className="flex items-center gap-2">
            <Radio className="h-4 w-4 text-gray-500 dark:text-gray-400" />
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

      {/* State the consequences before an apply that can block for a minute and
          roll back. */}
      <div className="rounded-lg border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-100">
        <p className="font-semibold">What happens when you apply</p>
        <ul className="mt-1 list-disc space-y-0.5 pl-5 text-amber-800 dark:text-amber-200">
          <li>The router switches to repeater mode and restarts the wireless subsystem.</li>
          <li>Your uplink moves to {upstream.ssid}; the current link drops.</li>
          <li>Devices must reconnect to the access point using its new name and password.</li>
          <li>If any step fails, earlier steps are rolled back automatically.</li>
          <li>Keep this page open until it finishes.</li>
        </ul>
      </div>

      {applyError && (
        <div className="rounded-md border border-red-200 bg-red-50 p-3 dark:border-red-900/50 dark:bg-red-950/30">
          {failedStep && (
            <p className="text-sm font-medium text-red-700 dark:text-red-300">
              Failed while applying: {failedStep}
            </p>
          )}
          <p className="text-sm text-red-600 dark:text-red-400" role="alert">
            {applyError}
          </p>
        </div>
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
