import type { ChangeEvent, RefObject } from 'react';
import type { FieldErrors, UseFormRegister } from 'react-hook-form';
import { Zap, Upload } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { formatBytes } from '@/lib/utils';
import type { FirmwareUpgradeFormValues } from '@/lib/schemas/system-forms';

type FirmwareUpgradeFormFieldsProps = {
  firmwareInputRef: RefObject<HTMLInputElement | null>;
  register: UseFormRegister<FirmwareUpgradeFormValues>;
  errors: FieldErrors<FirmwareUpgradeFormValues>;
  firmwareFile: unknown;
  onFirmwareInputChange: (e: ChangeEvent<HTMLInputElement>) => void;
  onSelectFirmwareClick: () => void;
  flashDisabled: boolean;
  flashPending: boolean;
  onOpenConfirmFlash: () => void;
};

export function FirmwareUpgradeFormFields({
  firmwareInputRef,
  register,
  errors,
  firmwareFile,
  onFirmwareInputChange,
  onSelectFirmwareClick,
  flashDisabled,
  flashPending,
  onOpenConfirmFlash,
}: FirmwareUpgradeFormFieldsProps) {
  return (
    <div className="space-y-3">
      <p className="text-xs text-gray-500">
        Upload a sysupgrade firmware image (.bin) to flash the device.
      </p>
      <div>
        <input
          type="file"
          ref={firmwareInputRef}
          accept=".bin"
          className="hidden"
          onChange={onFirmwareInputChange}
        />
        <Button type="button" variant="outline" size="sm" onClick={onSelectFirmwareClick}>
          <Upload className="mr-2 h-4 w-4" />
          Select Firmware Image
        </Button>
        {firmwareFile instanceof File && (
          <p className="mt-1 text-xs text-gray-700 dark:text-gray-300">
            Selected: {firmwareFile.name} ({formatBytes(firmwareFile.size)})
          </p>
        )}
        {errors.firmware ? (
          <p className="mt-1 text-xs text-red-500" role="alert">
            {errors.firmware.message}
          </p>
        ) : null}
      </div>
      <div className="flex items-center justify-between">
        <label htmlFor="keep-settings" className="text-sm text-gray-700 dark:text-gray-300">
          Keep current settings
        </label>
        <Switch id="keep-settings" label="Keep settings" {...register('keep_settings')} />
      </div>
      <Button
        type="button"
        variant="destructive"
        size="sm"
        disabled={flashDisabled}
        onClick={onOpenConfirmFlash}
      >
        <Zap className="mr-2 h-4 w-4" />
        {flashPending ? 'Flashing…' : 'Upload & Flash'}
      </Button>
    </div>
  );
}
