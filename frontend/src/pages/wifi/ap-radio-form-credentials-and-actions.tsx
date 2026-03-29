import {
  Controller,
  type Control,
  type FieldErrors,
  type UseFormRegister,
  type UseFormSetValue,
} from 'react-hook-form';
import { QrCode } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { InfoTooltip } from '@/components/ui/info-tooltip';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { APConfig } from '@shared/index';
import type { APRadioFormValues } from '@/lib/schemas/wifi-forms';

type ApRadioFormCredentialsAndActionsProps = {
  ap: APConfig;
  register: UseFormRegister<APRadioFormValues>;
  control: Control<APRadioFormValues>;
  errors: FieldErrors<APRadioFormValues>;
  encryption: string;
  setValue: UseFormSetValue<APRadioFormValues>;
  savePending: boolean;
  onOpenQr: () => void;
};

export function ApRadioFormCredentialsAndActions({
  ap,
  register,
  control,
  errors,
  encryption,
  setValue,
  savePending,
  onOpenQr,
}: ApRadioFormCredentialsAndActionsProps) {
  return (
    <>
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
          placeholder="Network name"
          aria-invalid={errors.ssid ? 'true' : undefined}
          aria-describedby={errors.ssid ? `ap-ssid-err-${ap.section}` : undefined}
          {...register('ssid')}
        />
        {errors.ssid ? (
          <p id={`ap-ssid-err-${ap.section}`} className="text-xs text-red-500" role="alert">
            {errors.ssid.message}
          </p>
        ) : null}
      </div>
      <div className="space-y-2">
        <label
          htmlFor={`ap-enc-${ap.section}`}
          className="text-xs font-medium text-gray-600 dark:text-gray-400"
        >
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
          )}
        />
      </div>
      {encryption !== 'none' && (
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
            placeholder="Minimum 8 characters"
            aria-invalid={errors.key ? 'true' : undefined}
            aria-describedby={errors.key ? `ap-key-err-${ap.section}` : undefined}
            {...register('key')}
          />
          {errors.key ? (
            <p id={`ap-key-err-${ap.section}`} className="text-xs text-red-500" role="alert">
              {errors.key.message}
            </p>
          ) : null}
        </div>
      )}
      <div className="flex gap-2">
        <Button type="submit" size="sm" disabled={savePending}>
          {savePending ? 'Saving...' : 'Save'}
        </Button>
        <Button type="button" variant="outline" size="sm" onClick={onOpenQr}>
          <QrCode className="mr-1 h-4 w-4" />
          QR Code
        </Button>
      </div>
    </>
  );
}
