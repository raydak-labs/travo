import { useRef, useState, type ChangeEvent } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Zap } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { useFirmwareUpgrade } from '@/hooks/use-system';
import {
  firmwareUpgradeFormSchema,
  type FirmwareUpgradeFormValues,
} from '@/lib/schemas/system-forms';
import { FirmwareUpgradeConfirmDialog } from '@/pages/system/firmware-upgrade-confirm-dialog';
import { FirmwareUpgradeFormFields } from '@/pages/system/firmware-upgrade-form-fields';

export function FirmwareUpgradeCard() {
  const firmwareUpgrade = useFirmwareUpgrade();
  const firmwareInputRef = useRef<HTMLInputElement>(null);
  const [showFirmwareDialog, setShowFirmwareDialog] = useState(false);

  const {
    register,
    setValue,
    watch,
    reset,
    getValues,
    formState: { errors },
  } = useForm<FirmwareUpgradeFormValues>({
    resolver: zodResolver(firmwareUpgradeFormSchema),
    defaultValues: {
      keep_settings: true,
      firmware: null,
    },
    mode: 'onChange',
  });

  const keepSettings = watch('keep_settings');
  const firmwareFile = watch('firmware');

  const onConfirmFlash = () => {
    const data = getValues();
    if (!(data.firmware instanceof File)) return;
    firmwareUpgrade.mutate(
      { file: data.firmware, keepSettings: data.keep_settings },
      {
        onSuccess: () => {
          reset({ keep_settings: true, firmware: null });
        },
      },
    );
    setShowFirmwareDialog(false);
  };

  const fileName = firmwareFile instanceof File ? firmwareFile.name : '';

  const onFirmwareInputChange = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      setValue('firmware', file, { shouldValidate: true, shouldDirty: true });
    }
    e.target.value = '';
  };

  return (
    <>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">Firmware Upgrade</CardTitle>
          <Zap className="h-4 w-4 text-gray-500" />
        </CardHeader>
        <CardContent>
          <FirmwareUpgradeFormFields
            firmwareInputRef={firmwareInputRef}
            register={register}
            errors={errors}
            firmwareFile={firmwareFile}
            onFirmwareInputChange={onFirmwareInputChange}
            onSelectFirmwareClick={() => firmwareInputRef.current?.click()}
            flashDisabled={!(firmwareFile instanceof File) || firmwareUpgrade.isPending}
            flashPending={firmwareUpgrade.isPending}
            onOpenConfirmFlash={() => setShowFirmwareDialog(true)}
          />
        </CardContent>
      </Card>

      <FirmwareUpgradeConfirmDialog
        open={showFirmwareDialog}
        onOpenChange={setShowFirmwareDialog}
        fileName={fileName}
        keepSettings={keepSettings}
        onConfirm={onConfirmFlash}
        confirmPending={firmwareUpgrade.isPending}
      />
    </>
  );
}
