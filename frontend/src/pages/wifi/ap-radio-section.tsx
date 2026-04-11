import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { WifiQRDialog } from '@/components/wifi/wifi-qr-dialog';
import { useSetAPConfig } from '@/hooks/use-wifi';
import type { APConfig, APConfigUpdate } from '@shared/index';
import { apRadioFormSchema, type APRadioFormValues } from '@/lib/schemas/wifi-forms';
import { normalizeApEncryption } from './ap-config-normalize';
import { ApRadioFormFields } from './ap-radio-form-fields';
import { ApRadioDisableDialog } from './ap-radio-disable-dialog';

export type APRadioSectionProps = {
  ap: APConfig;
  activeEnabledCount: number;
  onEnabledChange: (section: string, enabled: boolean) => void;
};

export function APRadioSection({ ap, activeEnabledCount, onEnabledChange }: APRadioSectionProps) {
  const setAP = useSetAPConfig();
  const [disableDialogOpen, setDisableDialogOpen] = useState(false);
  const [qrOpen, setQrOpen] = useState(false);
  const [qrPayload, setQrPayload] = useState<APConfig | null>(null);

  const {
    register,
    handleSubmit,
    control,
    reset,
    setValue,
    getValues,
    watch,
    formState: { errors },
  } = useForm<APRadioFormValues>({
    resolver: zodResolver(apRadioFormSchema),
    defaultValues: {
      enabled: ap.enabled,
      ssid: ap.ssid,
      encryption: normalizeApEncryption(ap.encryption),
      key: ap.key,
    },
    mode: 'onChange',
  });

  const enabled = watch('enabled');
  const encryption = watch('encryption');

  useEffect(() => {
    reset({
      enabled: ap.enabled,
      ssid: ap.ssid,
      encryption: normalizeApEncryption(ap.encryption),
      key: ap.key,
    });
  }, [ap.section, ap.enabled, ap.ssid, ap.encryption, ap.key, reset]);

  useEffect(() => {
    onEnabledChange(ap.section, enabled);
  }, [ap.section, enabled, onEnabledChange]);

  const buildConfig = (data: APRadioFormValues): APConfigUpdate => ({
    ssid: data.ssid.trim(),
    encryption: data.encryption,
    key: data.encryption === 'none' ? '' : data.key,
    enabled: data.enabled,
  });

  const onSubmit = (data: APRadioFormValues) => {
    if (ap.enabled && !data.enabled) {
      setDisableDialogOpen(true);
      return;
    }
    setAP.mutate({ section: ap.section, config: buildConfig(data) });
  };

  const confirmDisable = () => {
    const data = getValues();
    setAP.mutate({
      section: ap.section,
      config: { ...buildConfig(data), enabled: false },
    });
    setDisableDialogOpen(false);
  };

  const isLastActive = activeEnabledCount <= 1;
  const bandLabel = ap.band === '5g' ? '5 GHz' : ap.band === '2g' ? '2.4 GHz' : ap.band;

  const openQrFromForm = () => {
    const v = getValues();
    setQrPayload({
      ...ap,
      ssid: v.ssid.trim(),
      encryption: v.encryption,
      key: v.encryption === 'none' ? '' : v.key,
      enabled: v.enabled,
    });
    setQrOpen(true);
  };

  return (
    <>
      <form
        onSubmit={handleSubmit(onSubmit)}
        className="space-y-3 rounded-lg border p-4"
        noValidate
      >
        <ApRadioFormFields
          ap={ap}
          bandLabel={bandLabel}
          register={register}
          control={control}
          errors={errors}
          encryption={encryption}
          setValue={setValue}
          savePending={setAP.isPending}
          onOpenQr={openQrFromForm}
        />
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
          if (!open) setDisableDialogOpen(false);
        }}
        isLastActive={isLastActive}
        onConfirm={confirmDisable}
        confirmPending={setAP.isPending}
      />
    </>
  );
}
