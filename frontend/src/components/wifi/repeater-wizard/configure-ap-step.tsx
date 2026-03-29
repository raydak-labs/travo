import type { Dispatch, SetStateAction } from 'react';
import { Radio, ArrowRight, ArrowLeft } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { DialogFooter } from '@/components/ui/dialog';
import type { RepeaterUpstreamConfig, RepeaterApFormConfig } from './types';

type ConfigureApStepProps = {
  upstream: RepeaterUpstreamConfig;
  apConfig: RepeaterApFormConfig;
  setApConfig: Dispatch<SetStateAction<RepeaterApFormConfig>>;
  canProceedAP: boolean;
  onBack: () => void;
  onNext: () => void;
};

export function RepeaterWizardConfigureApStep({
  upstream,
  apConfig,
  setApConfig,
  canProceedAP,
  onBack,
  onNext,
}: ConfigureApStepProps) {
  return (
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
        <Button variant="outline" onClick={onBack}>
          <ArrowLeft className="mr-1.5 h-4 w-4" />
          Back
        </Button>
        <Button onClick={onNext} disabled={!canProceedAP}>
          Next
          <ArrowRight className="ml-1.5 h-4 w-4" />
        </Button>
      </DialogFooter>
    </div>
  );
}
