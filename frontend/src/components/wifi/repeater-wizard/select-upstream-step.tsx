import type { Dispatch, SetStateAction } from 'react';
import { ArrowRight } from 'lucide-react';
import type { WifiScanResult, GroupedScanNetwork } from '@shared/index';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { DialogFooter } from '@/components/ui/dialog';
import { SignalStrengthIcon } from '@/components/wifi/signal-strength-icon';
import { SecurityBadge } from '@/components/wifi/security-badge';
import { WifiScanList } from '@/pages/wifi/wifi-scan-list';
import type { RepeaterUpstreamConfig } from './types';

type SelectUpstreamStepProps = {
  selectedNetwork: WifiScanResult | null;
  upstream: RepeaterUpstreamConfig;
  setUpstream: Dispatch<SetStateAction<RepeaterUpstreamConfig>>;
  needsPassword: boolean;
  canProceedUpstream: boolean;
  scanResults: WifiScanResult[];
  scanLoading: boolean;
  onRefreshScan: () => void;
  onSelectNetwork: (group: GroupedScanNetwork) => void;
  onClearSelection: () => void;
  onNext: () => void;
  onCancel: () => void;
};

export function RepeaterWizardSelectUpstreamStep({
  selectedNetwork,
  upstream,
  setUpstream,
  needsPassword,
  canProceedUpstream,
  scanResults,
  scanLoading,
  onRefreshScan,
  onSelectNetwork,
  onClearSelection,
  onNext,
  onCancel,
}: SelectUpstreamStepProps) {
  return (
    <div className="space-y-4">
      {selectedNetwork ? (
        <div className="space-y-4">
          <div className="flex items-center gap-3 rounded-lg border p-3">
            <SignalStrengthIcon signalPercent={selectedNetwork.signal_percent} />
            <div>
              <p className="text-sm font-medium text-gray-900 dark:text-white">
                {selectedNetwork.ssid}
              </p>
              <SecurityBadge encryption={selectedNetwork.encryption} />
            </div>
            <Button variant="ghost" size="sm" className="ml-auto" onClick={onClearSelection}>
              Change
            </Button>
          </div>

          {needsPassword && (
            <div className="space-y-2">
              <label
                htmlFor="upstream-password"
                className="text-xs font-medium text-gray-600 dark:text-gray-400"
              >
                Password
              </label>
              <Input
                id="upstream-password"
                type="password"
                value={upstream.password}
                onChange={(e) => setUpstream((prev) => ({ ...prev, password: e.target.value }))}
                placeholder="Enter network password"
                autoFocus
              />
              {upstream.password.length > 0 && upstream.password.length < 8 && (
                <p className="text-xs text-red-500">Password must be at least 8 characters</p>
              )}
            </div>
          )}

          <DialogFooter>
            <Button variant="outline" onClick={onCancel}>
              Cancel
            </Button>
            <Button onClick={onNext} disabled={!canProceedUpstream}>
              Next
              <ArrowRight className="ml-1.5 h-4 w-4" />
            </Button>
          </DialogFooter>
        </div>
      ) : (
        <WifiScanList
          networks={scanResults}
          isLoading={scanLoading}
          onRefresh={onRefreshScan}
          onConnect={onSelectNetwork}
        />
      )}
    </div>
  );
}
