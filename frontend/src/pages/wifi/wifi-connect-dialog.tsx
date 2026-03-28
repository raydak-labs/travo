import { useMemo, useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import type { GroupedScanNetwork } from '@shared/index';
import {
  createWifiConnectFormSchema,
  type WifiConnectFormValues,
} from '@/lib/schemas/wifi-forms';
import {
  buildBandOptionsFromGroup,
  pickDefaultBandFromOptions,
  toWifiBand,
} from './wifi-connect-utils';
import { WifiConnectDialogForm } from './wifi-connect-dialog-form';

interface WifiConnectDialogProps {
  group: GroupedScanNetwork;
  isConnecting: boolean;
  error: string | null;
  onConnect: (ssid: string, password: string, band?: string) => void;
  onCancel: () => void;
  /** When true, renders inline without overlay */
  embedded?: boolean;
}

export function WifiConnectDialog({
  group,
  isConnecting,
  error,
  onConnect,
  onCancel,
  embedded,
}: WifiConnectDialogProps) {
  const [showPassword, setShowPassword] = useState(false);
  const needsPassword = group.encryption !== 'none';

  const bandOptions = useMemo(() => buildBandOptionsFromGroup(group), [group]);
  const defaultBand = useMemo(() => pickDefaultBandFromOptions(bandOptions), [bandOptions]);

  const formSchema = useMemo(() => createWifiConnectFormSchema(needsPassword), [needsPassword]);

  const {
    register,
    handleSubmit,
    reset,
    watch,
    formState: { errors },
  } = useForm<WifiConnectFormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: { password: '', selectedBand: '' },
    mode: 'onChange',
  });

  const passwordValue = watch('password');

  useEffect(() => {
    reset({
      password: '',
      selectedBand: defaultBand ?? '',
    });
  }, [group.ssid, group.encryption, defaultBand, reset]);

  const onValidSubmit = (data: WifiConnectFormValues) => {
    const band = data.selectedBand.trim() ? toWifiBand(data.selectedBand) : undefined;
    onConnect(group.ssid, data.password, band);
  };

  const content = (
    <WifiConnectDialogForm
      group={group}
      bandOptions={bandOptions}
      needsPassword={needsPassword}
      register={register}
      handleSubmit={handleSubmit}
      onValidSubmit={onValidSubmit}
      errors={errors}
      passwordValue={passwordValue}
      showPassword={showPassword}
      setShowPassword={setShowPassword}
      error={error}
      isConnecting={isConnecting}
      onCancel={onCancel}
    />
  );

  if (embedded) return content;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      role="dialog"
      aria-label="Connect to network"
    >
      <div className="mx-4 w-full max-w-md rounded-lg bg-white p-6 shadow-xl dark:bg-gray-900">
        {content}
      </div>
    </div>
  );
}
