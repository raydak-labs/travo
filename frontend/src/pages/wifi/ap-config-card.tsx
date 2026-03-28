import { useState, useEffect } from 'react';
import { Radio, QrCode } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { EmptyState } from '@/components/ui/empty-state';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { InfoTooltip } from '@/components/ui/info-tooltip';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { WifiQRDialog } from '@/components/wifi/wifi-qr-dialog';
import { useAPConfigs, useSetAPConfig } from '@/hooks/use-wifi';
import type { APConfig } from '@shared/index';

interface APFormState {
  ssid: string;
  encryption: string;
  key: string;
  enabled: boolean;
}

export function APConfigCard() {
  const { data: apConfigs, isLoading: apLoading } = useAPConfigs();
  const setAP = useSetAPConfig();
  const [apState, setApState] = useState<Record<string, APFormState>>({});
  const [qrAP, setQrAP] = useState<APConfig | null>(null);
  const [pendingDisableSection, setPendingDisableSection] = useState<string | null>(null);

  useEffect(() => {
    if (apConfigs) {
      const state: Record<string, APFormState> = {};
      for (const ap of apConfigs) {
        state[ap.section] = {
          ssid: ap.ssid,
          encryption: ap.encryption,
          key: ap.key,
          enabled: ap.enabled,
        };
      }
      setApState(state);
    }
  }, [apConfigs]);

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-medium">Access Point Configuration</CardTitle>
        </CardHeader>
        <CardContent>
          {apLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
            </div>
          ) : !apConfigs || apConfigs.length === 0 ? (
            <EmptyState message="No access point radios detected" />
          ) : (
            <div className="space-y-6">
              {apConfigs.map((ap) => {
                const form = apState[ap.section];
                if (!form) return null;
                const bandLabel =
                  ap.band === '5g' ? '5 GHz' : ap.band === '2g' ? '2.4 GHz' : ap.band;
                return (
                  <div key={ap.section} className="space-y-3 rounded-lg border p-4">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <Radio className="h-4 w-4 text-gray-500" />
                        <span className="text-sm font-medium text-gray-900 dark:text-white">
                          {ap.radio}
                        </span>
                        <Badge variant="outline">{bandLabel}</Badge>
                        <span className="text-xs text-gray-500">Ch {ap.channel}</span>
                      </div>
                      <Switch
                        id={`ap-enabled-${ap.section}`}
                        label="Enabled"
                        checked={form.enabled}
                        onChange={(e) =>
                          setApState((prev) => ({
                            ...prev,
                            [ap.section]: { ...prev[ap.section], enabled: e.target.checked },
                          }))
                        }
                      />
                    </div>
                    <div className="space-y-2">
                      <label
                        htmlFor={`ap-ssid-${ap.section}`}
                        className="flex items-center gap-1 text-xs font-medium text-gray-600 dark:text-gray-400"
                      >
                        SSID
                        <InfoTooltip text="The name of your WiFi network that devices see when scanning. Keep it descriptive but avoid including personal information." />
                      </label>
                      <Input
                        id={`ap-ssid-${ap.section}`}
                        value={form.ssid}
                        onChange={(e) =>
                          setApState((prev) => ({
                            ...prev,
                            [ap.section]: { ...prev[ap.section], ssid: e.target.value },
                          }))
                        }
                        placeholder="Network name"
                      />
                    </div>
                    <div className="space-y-2">
                      <label
                        htmlFor={`ap-enc-${ap.section}`}
                        className="text-xs font-medium text-gray-600 dark:text-gray-400"
                      >
                        Encryption
                      </label>
                      <Select
                        value={form.encryption}
                        onValueChange={(val) =>
                          setApState((prev) => ({
                            ...prev,
                            [ap.section]: {
                              ...prev[ap.section],
                              encryption: val,
                              key: val === 'none' ? '' : prev[ap.section].key,
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
                    {form.encryption !== 'none' && (
                      <div className="space-y-2">
                        <label
                          htmlFor={`ap-key-${ap.section}`}
                          className="flex items-center gap-1 text-xs font-medium text-gray-600 dark:text-gray-400"
                        >
                          Password
                          <InfoTooltip text="WiFi password (WPA key). Must be 8–63 characters for WPA2/WPA3. Avoid dictionary words — use a mix of letters, numbers, and symbols." />
                        </label>
                        <Input
                          id={`ap-key-${ap.section}`}
                          type="password"
                          value={form.key}
                          onChange={(e) =>
                            setApState((prev) => ({
                              ...prev,
                              [ap.section]: { ...prev[ap.section], key: e.target.value },
                            }))
                          }
                          placeholder="Minimum 8 characters"
                        />
                      </div>
                    )}
                    <div className="flex gap-2">
                      <Button
                        size="sm"
                        disabled={setAP.isPending}
                        onClick={() => {
                          if (ap.enabled && !form.enabled) {
                            setPendingDisableSection(ap.section);
                          } else {
                            setAP.mutate({
                              section: ap.section,
                              config: {
                                ...ap,
                                ssid: form.ssid,
                                encryption: form.encryption,
                                key: form.key,
                                enabled: form.enabled,
                              },
                            });
                          }
                        }}
                      >
                        {setAP.isPending ? 'Saving...' : 'Save'}
                      </Button>
                      <Button variant="outline" size="sm" onClick={() => setQrAP(ap)}>
                        <QrCode className="h-4 w-4 mr-1" />
                        QR Code
                      </Button>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>

      <WifiQRDialog
        open={qrAP !== null}
        onOpenChange={(open) => !open && setQrAP(null)}
        ap={qrAP}
      />

      {pendingDisableSection && (() => {
        const targetAP = apConfigs?.find((a) => a.section === pendingDisableSection);
        const targetForm = apState[pendingDisableSection];
        const activeAPCount = apConfigs?.filter((a) => {
          const s = apState[a.section];
          return s ? s.enabled : a.enabled;
        }).length ?? 0;
        const isLastActive = activeAPCount <= 1;
        return (
          <Dialog
            open={pendingDisableSection !== null}
            onOpenChange={(open) => !open && setPendingDisableSection(null)}
          >
            <DialogContent>
              <DialogHeader>
                <DialogTitle className="flex items-center gap-2">
                  {isLastActive ? '⚠️ Disable Last Access Point?' : 'Disable Access Point?'}
                </DialogTitle>
              </DialogHeader>
              {isLastActive ? (
                <p className="text-sm text-gray-700 dark:text-gray-300">
                  This is the <strong>only active access point</strong>. Disabling it will make the
                  router unreachable via WiFi. You will need a wired connection or physical access to
                  re-enable it.
                </p>
              ) : (
                <p className="text-sm text-gray-700 dark:text-gray-300">
                  Disabling this access point will disconnect all clients currently connected to it.
                  Are you sure?
                </p>
              )}
              <DialogFooter>
                <Button variant="outline" onClick={() => setPendingDisableSection(null)}>
                  Cancel
                </Button>
                <Button
                  variant="destructive"
                  onClick={() => {
                    if (targetAP && targetForm) {
                      setAP.mutate({
                        section: pendingDisableSection,
                        config: {
                          ...targetAP,
                          ssid: targetForm.ssid,
                          encryption: targetForm.encryption,
                          key: targetForm.key,
                          enabled: false,
                        },
                      });
                    }
                    setPendingDisableSection(null);
                  }}
                >
                  Disable
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        );
      })()}
    </>
  );
}
