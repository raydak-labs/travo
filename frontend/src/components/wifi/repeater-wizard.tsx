import { useState, useCallback } from 'react';
import { Wifi, Radio, CheckCircle2, Loader2, ArrowRight, ArrowLeft } from 'lucide-react';
import type { WifiScanResult } from '@shared/index';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { SignalStrengthIcon } from '@/components/wifi/signal-strength-icon';
import { SecurityBadge } from '@/components/wifi/security-badge';
import { WifiScanList } from '@/pages/wifi/wifi-scan-list';
import {
  useWifiScan,
  useWifiConnect,
  useWifiMode,
  useAPConfigs,
  useSetAPConfig,
} from '@/hooks/use-wifi';

type WizardStep = 'select-upstream' | 'configure-ap' | 'review';

interface UpstreamConfig {
  ssid: string;
  password: string;
  encryption: string;
}

interface APFormConfig {
  ssid: string;
  encryption: string;
  key: string;
  sameAsUpstream: boolean;
}

interface RepeaterWizardProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function RepeaterWizard({ open, onOpenChange }: RepeaterWizardProps) {
  const [step, setStep] = useState<WizardStep>('select-upstream');
  const [selectedNetwork, setSelectedNetwork] = useState<WifiScanResult | null>(null);
  const [upstream, setUpstream] = useState<UpstreamConfig>({
    ssid: '',
    password: '',
    encryption: '',
  });
  const [apConfig, setApConfig] = useState<APFormConfig>({
    ssid: '',
    encryption: 'psk2',
    key: '',
    sameAsUpstream: true,
  });
  const [applying, setApplying] = useState(false);
  const [applyError, setApplyError] = useState<string | null>(null);
  const [done, setDone] = useState(false);

  const { data: scanResults = [], isLoading: scanLoading, refetch } = useWifiScan(open);
  const { data: apConfigs } = useAPConfigs();
  const connectMutation = useWifiConnect();
  const modeMutation = useWifiMode();
  const setAPMutation = useSetAPConfig();

  const reset = useCallback(() => {
    setStep('select-upstream');
    setSelectedNetwork(null);
    setUpstream({ ssid: '', password: '', encryption: '' });
    setApConfig({ ssid: '', encryption: 'psk2', key: '', sameAsUpstream: true });
    setApplying(false);
    setApplyError(null);
    setDone(false);
  }, []);

  function handleClose(isOpen: boolean) {
    if (!isOpen) {
      reset();
    }
    onOpenChange(isOpen);
  }

  function handleSelectNetwork(network: WifiScanResult) {
    setSelectedNetwork(network);
    setUpstream({
      ssid: network.ssid,
      password: '',
      encryption: network.encryption,
    });
    setApConfig((prev) => ({
      ...prev,
      ssid: prev.sameAsUpstream ? network.ssid : prev.ssid,
    }));
  }

  function handleUpstreamNext() {
    setStep('configure-ap');
  }

  function handleAPNext() {
    setStep('review');
  }

  const effectiveAPSSID = apConfig.sameAsUpstream ? upstream.ssid : apConfig.ssid;
  const effectiveAPKey = apConfig.sameAsUpstream ? upstream.password : apConfig.key;
  const effectiveAPEncryption = apConfig.sameAsUpstream
    ? mapEncryption(upstream.encryption)
    : apConfig.encryption;

  async function handleApply() {
    setApplying(true);
    setApplyError(null);

    try {
      // Step 1: Set mode to repeater
      await modeMutation.mutateAsync('repeater');

      // Step 2: Connect to upstream
      await connectMutation.mutateAsync({
        ssid: upstream.ssid,
        password: upstream.password,
        encryption: upstream.encryption,
      });

      // Step 3: Configure AP(s)
      if (apConfigs && apConfigs.length > 0) {
        const ap = apConfigs[0];
        await setAPMutation.mutateAsync({
          section: ap.section,
          config: {
            ...ap,
            ssid: effectiveAPSSID,
            encryption: effectiveAPEncryption,
            key: effectiveAPKey,
            enabled: true,
          },
        });
      }

      setDone(true);
    } catch (err) {
      setApplyError(err instanceof Error ? err.message : 'Setup failed');
    } finally {
      setApplying(false);
    }
  }

  const needsPassword = selectedNetwork?.encryption !== 'none';
  const canProceedUpstream =
    selectedNetwork != null && (!needsPassword || upstream.password.length >= 8);
  const canProceedAP =
    apConfig.sameAsUpstream ||
    (apConfig.ssid.length > 0 && (apConfig.encryption === 'none' || apConfig.key.length >= 8));

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Repeater Setup Wizard</DialogTitle>
          <DialogDescription>Set up your router as a WiFi repeater in 3 steps.</DialogDescription>
        </DialogHeader>

        {/* Step indicator */}
        <div className="flex items-center justify-center gap-2 py-2">
          {(['select-upstream', 'configure-ap', 'review'] as WizardStep[]).map((s, i) => {
            const labels = ['Upstream', 'AP Config', 'Review'];
            const isActive = s === step;
            const stepIndex = ['select-upstream', 'configure-ap', 'review'].indexOf(step);
            const isPast = i < stepIndex;
            return (
              <div key={s} className="flex items-center gap-2">
                {i > 0 && (
                  <div
                    className={`h-px w-6 ${isPast || isActive ? 'bg-blue-500' : 'bg-gray-300 dark:bg-gray-700'}`}
                  />
                )}
                <div className="flex flex-col items-center gap-1">
                  <div
                    className={`flex h-7 w-7 items-center justify-center rounded-full text-xs font-medium ${
                      isActive
                        ? 'bg-blue-600 text-white'
                        : isPast
                          ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300'
                          : 'bg-gray-200 text-gray-500 dark:bg-gray-800 dark:text-gray-400'
                    }`}
                  >
                    {i + 1}
                  </div>
                  <span className="text-xs text-gray-500">{labels[i]}</span>
                </div>
              </div>
            );
          })}
        </div>

        {/* Step 1: Select upstream */}
        {step === 'select-upstream' && (
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
                  <Button
                    variant="ghost"
                    size="sm"
                    className="ml-auto"
                    onClick={() => setSelectedNetwork(null)}
                  >
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
                      onChange={(e) =>
                        setUpstream((prev) => ({ ...prev, password: e.target.value }))
                      }
                      placeholder="Enter network password"
                      autoFocus
                    />
                    {upstream.password.length > 0 && upstream.password.length < 8 && (
                      <p className="text-xs text-red-500">Password must be at least 8 characters</p>
                    )}
                  </div>
                )}

                <DialogFooter>
                  <Button variant="outline" onClick={() => handleClose(false)}>
                    Cancel
                  </Button>
                  <Button onClick={handleUpstreamNext} disabled={!canProceedUpstream}>
                    Next
                    <ArrowRight className="ml-1.5 h-4 w-4" />
                  </Button>
                </DialogFooter>
              </div>
            ) : (
              <WifiScanList
                networks={scanResults}
                isLoading={scanLoading}
                onRefresh={() => void refetch()}
                onConnect={handleSelectNetwork}
              />
            )}
          </div>
        )}

        {/* Step 2: Configure AP */}
        {step === 'configure-ap' && (
          <div className="space-y-4">
            <div className="flex items-center gap-3 rounded-lg border border-blue-200 bg-blue-50 p-3 dark:border-blue-800 dark:bg-blue-950">
              <Radio className="h-4 w-4 text-blue-600" />
              <p className="text-sm text-blue-800 dark:text-blue-200">
                Configure the access point that your devices will connect to.
              </p>
            </div>

            <div className="flex items-center justify-between rounded-lg border p-3">
              <div className="space-y-0.5">
                <span className="text-sm font-medium text-gray-900 dark:text-white">
                  Same as upstream
                </span>
                <p className="text-xs text-gray-500">
                  Use the same SSID and password as the upstream network
                </p>
              </div>
              <Switch
                id="same-as-upstream"
                label="Same as upstream"
                checked={apConfig.sameAsUpstream}
                onChange={(e) =>
                  setApConfig((prev) => ({
                    ...prev,
                    sameAsUpstream: e.target.checked,
                    ssid: e.target.checked ? upstream.ssid : prev.ssid,
                  }))
                }
              />
            </div>

            {!apConfig.sameAsUpstream && (
              <div className="space-y-3 rounded-lg border p-4">
                <div className="space-y-2">
                  <label
                    htmlFor="ap-ssid"
                    className="text-xs font-medium text-gray-600 dark:text-gray-400"
                  >
                    AP SSID
                  </label>
                  <Input
                    id="ap-ssid"
                    value={apConfig.ssid}
                    onChange={(e) => setApConfig((prev) => ({ ...prev, ssid: e.target.value }))}
                    placeholder="Network name for your devices"
                  />
                </div>

                <div className="space-y-2">
                  <label
                    htmlFor="ap-encryption"
                    className="text-xs font-medium text-gray-600 dark:text-gray-400"
                  >
                    Encryption
                  </label>
                  <Select
                    value={apConfig.encryption}
                    onValueChange={(val) =>
                      setApConfig((prev) => ({
                        ...prev,
                        encryption: val,
                        key: val === 'none' ? '' : prev.key,
                      }))
                    }
                  >
                    <SelectTrigger id="ap-encryption">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="none">None (Open)</SelectItem>
                      <SelectItem value="psk2">WPA2-PSK</SelectItem>
                      <SelectItem value="sae">WPA3-SAE</SelectItem>
                      <SelectItem value="psk-mixed">WPA2/WPA3 Mixed</SelectItem>
                    </SelectContent>
                  </Select>
                </div>

                {apConfig.encryption !== 'none' && (
                  <div className="space-y-2">
                    <label
                      htmlFor="ap-key"
                      className="text-xs font-medium text-gray-600 dark:text-gray-400"
                    >
                      Password
                    </label>
                    <Input
                      id="ap-key"
                      type="password"
                      value={apConfig.key}
                      onChange={(e) => setApConfig((prev) => ({ ...prev, key: e.target.value }))}
                      placeholder="Minimum 8 characters"
                    />
                    {apConfig.key.length > 0 && apConfig.key.length < 8 && (
                      <p className="text-xs text-red-500">Password must be at least 8 characters</p>
                    )}
                  </div>
                )}
              </div>
            )}

            <DialogFooter>
              <Button variant="outline" onClick={() => setStep('select-upstream')}>
                <ArrowLeft className="mr-1.5 h-4 w-4" />
                Back
              </Button>
              <Button onClick={handleAPNext} disabled={!canProceedAP}>
                Next
                <ArrowRight className="ml-1.5 h-4 w-4" />
              </Button>
            </DialogFooter>
          </div>
        )}

        {/* Step 3: Review & Apply */}
        {step === 'review' && !done && (
          <div className="space-y-4">
            <div className="space-y-3">
              <h3 className="text-sm font-semibold text-gray-900 dark:text-white">
                Upstream Connection
              </h3>
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
                  <span className="text-sm font-medium">{effectiveAPSSID}</span>
                  <Badge variant="outline">
                    {effectiveAPEncryption === 'none'
                      ? 'Open'
                      : effectiveAPEncryption.toUpperCase()}
                  </Badge>
                </div>
                {apConfig.sameAsUpstream && (
                  <p className="mt-1 text-xs text-gray-500">Same credentials as upstream</p>
                )}
              </div>
            </div>

            {applyError && (
              <p className="text-sm text-red-600 dark:text-red-400" role="alert">
                {applyError}
              </p>
            )}

            <DialogFooter>
              <Button variant="outline" onClick={() => setStep('configure-ap')} disabled={applying}>
                <ArrowLeft className="mr-1.5 h-4 w-4" />
                Back
              </Button>
              <Button onClick={() => void handleApply()} disabled={applying}>
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
        )}

        {/* Done */}
        {done && (
          <div className="flex flex-col items-center gap-4 py-6">
            <CheckCircle2 className="h-12 w-12 text-green-500" />
            <div className="text-center">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
                Repeater Setup Complete
              </h3>
              <p className="mt-1 text-sm text-gray-500">
                Your router is now repeating <strong>{upstream.ssid}</strong> as{' '}
                <strong>{effectiveAPSSID}</strong>.
              </p>
            </div>
            <Button onClick={() => handleClose(false)}>Done</Button>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

/** Map scan encryption names to UCI encryption values */
function mapEncryption(enc: string): string {
  switch (enc) {
    case 'wpa2':
      return 'psk2';
    case 'wpa3':
      return 'sae';
    case 'wpa2/wpa3':
      return 'psk-mixed';
    case 'wpa':
      return 'psk';
    case 'none':
      return 'none';
    default:
      return 'psk2';
  }
}
