import { useEffect, useMemo, useState } from 'react';
import { useForm, Controller } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { QrCode, Radio } from 'lucide-react';
import type { APConfig, APConfigUpdate } from '@shared/index';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { InfoTooltip } from '@/components/ui/info-tooltip';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { unifiedApCredentialsSchema, type UnifiedApCredentialsValues } from '@/lib/schemas/wifi-forms';
import { useSetAPConfig } from '@/hooks/use-wifi';
import { WifiQRDialog } from '@/components/wifi/wifi-qr-dialog';
import { normalizeApEncryption } from './ap-config-normalize';
import { ApRadioDisableDialog } from './ap-radio-disable-dialog';

function bandLabel(band: string): string {
  if (band === '5g') return '5 GHz';
  if (band === '2g') return '2.4 GHz';
  if (band === '6g') return '6 GHz';
  return band;
}

function bandSortKey(band: string): number {
  if (band === '2g') return 0;
  if (band === '5g') return 1;
  if (band === '6g') return 2;
  return 9;
}

function pickPrimaryAp(aps: APConfig[]): APConfig {
  return [...aps].sort((a, b) => bandSortKey(a.band) - bandSortKey(b.band))[0]!;
}

function credentialsMatchAcross(aps: APConfig[]): boolean {
  if (aps.length <= 1) return true;
  const p = pickPrimaryAp(aps);
  const enc0 = normalizeApEncryption(p.encryption);
  return aps.every(
    (a) =>
      a.ssid === p.ssid &&
      normalizeApEncryption(a.encryption) === enc0 &&
      a.key === p.key,
  );
}

type APUnifiedConfigFormProps = {
  apConfigs: APConfig[];
  enabledBySection: Record<string, boolean>;
  activeEnabledCount: number;
  onEnabledChange: (section: string, enabled: boolean) => void;
};

export function APUnifiedConfigForm({
  apConfigs,
  enabledBySection,
  activeEnabledCount,
  onEnabledChange,
}: APUnifiedConfigFormProps) {
  const setAP = useSetAPConfig();
  const [qrOpen, setQrOpen] = useState(false);
  const [qrPayload, setQrPayload] = useState<APConfig | null>(null);
  const [disableDialogOpen, setDisableDialogOpen] = useState(false);
  const [pendingApply, setPendingApply] = useState<(() => void) | null>(null);

  const primary = useMemo(() => pickPrimaryAp(apConfigs), [apConfigs]);
  const mismatch = useMemo(() => !credentialsMatchAcross(apConfigs), [apConfigs]);

  const {
    register,
    handleSubmit,
    control,
    reset,
    setValue,
    getValues,
    watch,
    formState: { errors },
  } = useForm<UnifiedApCredentialsValues>({
    resolver: zodResolver(unifiedApCredentialsSchema),
    defaultValues: {
      ssid: primary.ssid,
      encryption: normalizeApEncryption(primary.encryption),
      key: primary.key,
    },
    mode: 'onChange',
  });

  const encryption = watch('encryption');

  useEffect(() => {
    const p = pickPrimaryAp(apConfigs);
    reset({
      ssid: p.ssid,
      encryption: normalizeApEncryption(p.encryption),
      key: p.key,
    });
  }, [apConfigs, reset]);

  const buildSharedUpdate = (data: UnifiedApCredentialsValues): Omit<APConfigUpdate, 'enabled'> => ({
    ssid: data.ssid.trim(),
    encryption: data.encryption,
    key: data.encryption === 'none' ? '' : data.key,
  });

  const applyAll = async (data: UnifiedApCredentialsValues) => {
    const shared = buildSharedUpdate(data);
    for (const ap of apConfigs) {
      const enabled = enabledBySection[ap.section] ?? ap.enabled;
      await setAP.mutateAsync({
        section: ap.section,
        config: { ...shared, enabled },
      });
    }
  };

  const onSubmit = (data: UnifiedApCredentialsValues) => {
    if (activeEnabledCount < 1) {
      if (!apConfigs.some((ap) => ap.enabled)) return;
      setPendingApply(() => () => {
        void applyAll(data).finally(() => {
          setPendingApply(null);
          setDisableDialogOpen(false);
        });
      });
      setDisableDialogOpen(true);
      return;
    }

    void applyAll(data);
  };

  const confirmDisable = () => {
    if (pendingApply) {
      pendingApply();
    }
  };

  const openQrFromForm = () => {
    const v = getValues();
    const p = pickPrimaryAp(apConfigs);
    setQrPayload({
      ...p,
      ssid: v.ssid.trim(),
      encryption: v.encryption,
      key: v.encryption === 'none' ? '' : v.key,
      enabled: enabledBySection[p.section] ?? p.enabled,
    });
    setQrOpen(true);
  };

  return (
    <>
      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4" noValidate>
        {mismatch ? (
          <p className="text-xs text-amber-700 dark:text-amber-300">
            Bands currently use different names or passwords. Showing {bandLabel(primary.band)}; saving
            applies these values to all bands.
          </p>
        ) : null}

        <div className="space-y-3 rounded-lg border p-4">
          {apConfigs.map((ap) => (
            <div key={ap.section} className="flex items-center justify-between border-b pb-3 last:border-0 last:pb-0">
              <div className="flex items-center gap-2">
                <Radio className="h-4 w-4 text-gray-500" />
                <span className="text-sm font-medium text-gray-900 dark:text-white">{ap.radio}</span>
                <Badge variant="outline">{bandLabel(ap.band)}</Badge>
                <span className="text-xs text-gray-500">Ch {ap.channel}</span>
              </div>
              <Switch
                id={`ap-unified-enabled-${ap.section}`}
                label="Enabled"
                checked={enabledBySection[ap.section] ?? ap.enabled}
                onChange={(e) => onEnabledChange(ap.section, e.target.checked)}
              />
            </div>
          ))}
        </div>

        <div className="space-y-2">
          <label
            htmlFor="ap-unified-ssid"
            className="flex items-center gap-1 text-xs font-medium text-gray-600 dark:text-gray-400"
          >
            Network name (all bands)
            <InfoTooltip text="The name of your WiFi network that devices see when scanning. Keep it descriptive but avoid including personal information." />
          </label>
          <Input
            id="ap-unified-ssid"
            placeholder="SSID for all radios"
            aria-invalid={errors.ssid ? 'true' : undefined}
            {...register('ssid')}
          />
          {errors.ssid ? (
            <p className="text-xs text-red-500" role="alert">
              {errors.ssid.message}
            </p>
          ) : null}
        </div>

        <div className="space-y-2">
          <label htmlFor="ap-unified-enc" className="text-xs font-medium text-gray-600 dark:text-gray-400">
            Encryption
          </label>
          <Controller
            name="encryption"
            control={control}
            render={({ field }) => (
              <Select
                value={field.value}
                onValueChange={(val) => {
                  field.onChange(val);
                  if (val === 'none') {
                    setValue('key', '', { shouldValidate: true });
                  }
                }}
              >
                <SelectTrigger id="ap-unified-enc">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">None (Open)</SelectItem>
                  <SelectItem value="psk2">WPA2-PSK</SelectItem>
                  <SelectItem value="sae">WPA3-SAE</SelectItem>
                  <SelectItem value="psk-mixed">WPA2/WPA3 Mixed</SelectItem>
                </SelectContent>
              </Select>
            )}
          />
        </div>

        {encryption !== 'none' && (
          <div className="space-y-2">
            <label
              htmlFor="ap-unified-key"
              className="flex items-center gap-1 text-xs font-medium text-gray-600 dark:text-gray-400"
            >
              Password
              <InfoTooltip text="WiFi password (WPA key). Must be 8–63 characters for WPA2/WPA3. Avoid dictionary words — use a mix of letters, numbers, and symbols." />
            </label>
            <Input
              id="ap-unified-key"
              type="password"
              placeholder="Minimum 8 characters"
              aria-invalid={errors.key ? 'true' : undefined}
              {...register('key')}
            />
            {errors.key ? (
              <p className="text-xs text-red-500" role="alert">
                {errors.key.message}
              </p>
            ) : null}
          </div>
        )}

        <div className="flex gap-2">
          <Button type="submit" size="sm" disabled={setAP.isPending}>
            {setAP.isPending ? 'Saving...' : 'Save'}
          </Button>
          <Button type="button" variant="outline" size="sm" onClick={openQrFromForm}>
            <QrCode className="mr-1 h-4 w-4" />
            QR Code
          </Button>
        </div>
      </form>

      <WifiQRDialog
        open={qrOpen}
        onOpenChange={(open) => {
          setQrOpen(open);
          if (!open) setQrPayload(null);
        }}
        ap={qrPayload}
      />

      <ApRadioDisableDialog
        open={disableDialogOpen}
        onOpenChange={(open) => {
          if (!open) {
            setDisableDialogOpen(false);
            setPendingApply(null);
          }
        }}
        isLastActive
        onConfirm={confirmDisable}
        confirmPending={setAP.isPending}
      />
    </>
  );
}
