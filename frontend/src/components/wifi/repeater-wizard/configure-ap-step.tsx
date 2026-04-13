import type { Dispatch, SetStateAction } from 'react';
import { Radio, ArrowRight, ArrowLeft } from 'lucide-react';
import type { APConfig } from '@shared/index';
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
import { mapScanEncryptionToUci } from './map-encryption';

type ConfigureApStepProps = {
  upstream: RepeaterUpstreamConfig;
  apConfig: RepeaterApFormConfig;
  setApConfig: Dispatch<SetStateAction<RepeaterApFormConfig>>;
  apConfigs: APConfig[] | undefined;
  allowApOnStaRadio: boolean;
  setAllowApOnStaRadio: (v: boolean) => void;
  canProceedAP: boolean;
  onBack: () => void;
  onNext: () => void;
};

function bandLabel(band: string): string {
  if (band === '2g') return '2.4 GHz';
  if (band === '5g') return '5 GHz';
  if (band === '6g') return '6 GHz';
  return band;
}

export function RepeaterWizardConfigureApStep({
  upstream,
  apConfig,
  setApConfig,
  apConfigs,
  allowApOnStaRadio,
  setAllowApOnStaRadio,
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
          <p className="text-xs text-gray-500 dark:text-gray-400">
            Use the same SSID and password as the upstream network for all bands
          </p>
        </div>
        <Switch
          id="same-as-upstream"
          label="Same as upstream"
          checked={apConfig.sameAsUpstream}
          onChange={(e) => {
            const checked = e.target.checked;
            setApConfig((prev) => ({
              ...prev,
              sameAsUpstream: checked,
              separateBandConfig: checked ? false : prev.separateBandConfig,
              perBand: checked ? {} : prev.perBand,
              ssid: checked ? upstream.ssid : prev.ssid,
            }));
          }}
        />
      </div>

      {!apConfig.sameAsUpstream && (
        <>
          {!apConfig.separateBandConfig && (
            <div className="space-y-3 rounded-lg border p-4">
              <div className="space-y-2">
                <label
                  htmlFor="ap-ssid-shared"
                  className="text-xs font-medium text-gray-600 dark:text-gray-400"
                >
                  Network name (all bands)
                </label>
                <Input
                  id="ap-ssid-shared"
                  value={apConfig.ssid}
                  onChange={(e) => setApConfig((prev) => ({ ...prev, ssid: e.target.value }))}
                  placeholder="SSID for 2.4 GHz and 5 GHz"
                />
              </div>

              <div className="space-y-2">
                <label
                  htmlFor="ap-encryption-shared"
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
                  <SelectTrigger id="ap-encryption-shared">
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
                    htmlFor="ap-key-shared"
                    className="text-xs font-medium text-gray-600 dark:text-gray-400"
                  >
                    Password
                  </label>
                  <Input
                    id="ap-key-shared"
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

          <div className="flex items-center justify-between rounded-lg border p-3">
            <div className="space-y-0.5">
              <span className="text-sm font-medium text-gray-900 dark:text-white">
                Different settings per radio
              </span>
              <p className="text-xs text-gray-500 dark:text-gray-400">
                Set a separate SSID and password for 2.4 GHz and 5 GHz
              </p>
            </div>
            <Switch
              id="separate-band"
              label="Different settings per radio"
              checked={apConfig.separateBandConfig}
              onChange={(e) => {
                const on = e.target.checked;
                setApConfig((prev) => {
                  if (!on) {
                    return { ...prev, separateBandConfig: false, perBand: {} };
                  }
                  const nextPer: RepeaterApFormConfig['perBand'] = {};
                  for (const ap of apConfigs ?? []) {
                    const ssid = prev.ssid.trim() || upstream.ssid;
                    const enc = mapScanEncryptionToUci(upstream.encryption) || prev.encryption;
                    const key = prev.key || upstream.password;
                    nextPer[ap.section] = {
                      ssid,
                      encryption: enc || 'psk2',
                      key: enc === 'none' ? '' : key,
                    };
                  }
                  return { ...prev, separateBandConfig: true, perBand: nextPer };
                });
              }}
            />
          </div>

          {apConfig.separateBandConfig &&
            (apConfigs ?? []).map((ap) => {
              const pb = apConfig.perBand[ap.section] ?? {
                ssid: '',
                encryption: 'psk2',
                key: '',
              };
              return (
                <div key={ap.section} className="space-y-3 rounded-lg border p-4">
                  <p className="text-sm font-medium">{bandLabel(ap.band)}</p>
                  <div className="space-y-2">
                    <label
                      className="text-xs font-medium text-gray-600 dark:text-gray-400"
                      htmlFor={`ap-ssid-${ap.section}`}
                    >
                      SSID
                    </label>
                    <Input
                      id={`ap-ssid-${ap.section}`}
                      value={pb.ssid}
                      onChange={(e) =>
                        setApConfig((prev) => ({
                          ...prev,
                          perBand: {
                            ...prev.perBand,
                            [ap.section]: { ...pb, ssid: e.target.value },
                          },
                        }))
                      }
                    />
                  </div>
                  <div className="space-y-2">
                    <label
                      className="text-xs font-medium text-gray-600 dark:text-gray-400"
                      htmlFor={`ap-enc-${ap.section}`}
                    >
                      Encryption
                    </label>
                    <Select
                      value={pb.encryption}
                      onValueChange={(val) =>
                        setApConfig((prev) => ({
                          ...prev,
                          perBand: {
                            ...prev.perBand,
                            [ap.section]: {
                              ...pb,
                              encryption: val,
                              key: val === 'none' ? '' : pb.key,
                            },
                          },
                        }))
                      }
                    >
                      <SelectTrigger id={`ap-enc-${ap.section}`}>
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
                  {pb.encryption !== 'none' && (
                    <div className="space-y-2">
                      <label
                        className="text-xs font-medium text-gray-600 dark:text-gray-400"
                        htmlFor={`ap-key-${ap.section}`}
                      >
                        Password
                      </label>
                      <Input
                        id={`ap-key-${ap.section}`}
                        type="password"
                        value={pb.key}
                        onChange={(e) =>
                          setApConfig((prev) => ({
                            ...prev,
                            perBand: {
                              ...prev.perBand,
                              [ap.section]: { ...pb, key: e.target.value },
                            },
                          }))
                        }
                      />
                    </div>
                  )}
                </div>
              );
            })}
        </>
      )}

      <div className="flex items-center justify-between rounded-lg border p-3">
        <div className="space-y-0.5">
          <span className="text-sm font-medium text-gray-900 dark:text-white">
            Allow Wi‑Fi on uplink radio
          </span>
          <p className="text-xs text-gray-500 dark:text-gray-400">
            Off (default): downlink AP only on the other radio when possible — more stable. On: also
            broadcast on the same radio as the client link (weaker on many chipsets).
          </p>
        </div>
        <Switch
          id="allow-ap-sta-radio"
          label="Allow Wi‑Fi on uplink radio"
          checked={allowApOnStaRadio}
          onChange={(e) => setAllowApOnStaRadio(e.target.checked)}
        />
      </div>

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
