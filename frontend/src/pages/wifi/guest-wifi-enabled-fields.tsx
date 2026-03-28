import { Controller, type Control, type FieldErrors, type UseFormRegister, type UseFormSetValue } from 'react-hook-form';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { GuestWifiFormValues } from '@/lib/schemas/wifi-forms';

type GuestWifiEnabledFieldsProps = {
  register: UseFormRegister<GuestWifiFormValues>;
  control: Control<GuestWifiFormValues>;
  errors: FieldErrors<GuestWifiFormValues>;
  encryption: string;
  setValue: UseFormSetValue<GuestWifiFormValues>;
};

export function GuestWifiEnabledFields({
  register,
  control,
  errors,
  encryption,
  setValue,
}: GuestWifiEnabledFieldsProps) {
  return (
    <div className="space-y-3 rounded-lg border p-4">
      <div className="space-y-2">
        <label
          htmlFor="guest-ssid"
          className="text-xs font-medium text-gray-600 dark:text-gray-400"
        >
          SSID
        </label>
        <Input
          id="guest-ssid"
          placeholder="Guest network name"
          aria-invalid={errors.ssid ? 'true' : undefined}
          aria-describedby={errors.ssid ? 'guest-ssid-err' : undefined}
          {...register('ssid')}
        />
        {errors.ssid ? (
          <p id="guest-ssid-err" className="text-xs text-red-500" role="alert">
            {errors.ssid.message}
          </p>
        ) : null}
      </div>
      <div className="space-y-2">
        <label
          htmlFor="guest-encryption"
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
              <SelectTrigger id="guest-encryption">
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
          <label htmlFor="guest-key" className="text-xs font-medium text-gray-600 dark:text-gray-400">
            Password
          </label>
          <Input
            id="guest-key"
            type="password"
            placeholder="Minimum 8 characters"
            aria-invalid={errors.key ? 'true' : undefined}
            aria-describedby={errors.key ? 'guest-key-err' : undefined}
            {...register('key')}
          />
          {errors.key ? (
            <p id="guest-key-err" className="text-xs text-red-500" role="alert">
              {errors.key.message}
            </p>
          ) : null}
        </div>
      )}
      <p className="text-xs text-gray-500">
        Client isolation is enabled — guests cannot see each other. Internet access only, no LAN
        access.
      </p>
    </div>
  );
}
