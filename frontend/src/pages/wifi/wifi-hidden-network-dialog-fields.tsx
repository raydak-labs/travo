import { Eye, EyeOff } from 'lucide-react';
import { Controller, type Control, type FieldErrors, type UseFormRegister } from 'react-hook-form';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import type { WifiHiddenNetworkFormValues } from '@/lib/schemas/wifi-forms';
import { WIFI_HIDDEN_ENCRYPTION_OPTIONS } from '@/pages/wifi/wifi-hidden-network-constants';

type WifiHiddenNetworkDialogFieldsProps = {
  register: UseFormRegister<WifiHiddenNetworkFormValues>;
  control: Control<WifiHiddenNetworkFormValues>;
  errors: FieldErrors<WifiHiddenNetworkFormValues>;
  needsPassword: boolean;
  showPassword: boolean;
  setShowPassword: (next: boolean) => void;
  errorMessage: string | null;
};

export function WifiHiddenNetworkDialogFields({
  register,
  control,
  errors,
  needsPassword,
  showPassword,
  setShowPassword,
  errorMessage,
}: WifiHiddenNetworkDialogFieldsProps) {
  return (
    <>
      <Input
        id="hidden-ssid"
        label="Network Name (SSID)"
        type="text"
        autoFocus
        aria-required="true"
        aria-invalid={errors.ssid ? 'true' : undefined}
        aria-describedby={errors.ssid ? 'hidden-ssid-error' : undefined}
        {...register('ssid')}
        placeholder="Enter network name"
      />
      {errors.ssid ? (
        <p id="hidden-ssid-error" className="text-sm text-red-600 dark:text-red-400" role="alert">
          {errors.ssid.message}
        </p>
      ) : null}

      <div className="space-y-1.5">
        <label
          htmlFor="hidden-encryption"
          className="text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          Encryption
        </label>
        <Controller
          control={control}
          name="encryption"
          render={({ field }) => (
            <Select value={field.value} onValueChange={field.onChange}>
              <SelectTrigger id="hidden-encryption">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {WIFI_HIDDEN_ENCRYPTION_OPTIONS.map((opt) => (
                  <SelectItem key={opt.value} value={opt.value}>
                    {opt.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        />
      </div>

      {needsPassword && (
        <div className="relative">
          <Input
            id="hidden-password"
            label="Password"
            type={showPassword ? 'text' : 'password'}
            aria-required="true"
            aria-invalid={errors.password ? 'true' : undefined}
            aria-describedby={errors.password ? 'hidden-password-error' : undefined}
            {...register('password')}
            placeholder="Enter network password"
          />
          <button
            type="button"
            className="absolute right-3 top-8 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
            onClick={() => setShowPassword(!showPassword)}
            aria-label={showPassword ? 'Hide password' : 'Show password'}
          >
            {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
          </button>
          {errors.password ? (
            <p
              id="hidden-password-error"
              className="mt-1 text-sm text-red-600 dark:text-red-400"
              role="alert"
            >
              {errors.password.message}
            </p>
          ) : null}
        </div>
      )}

      {errorMessage ? (
        <p className="text-sm text-red-600 dark:text-red-400" role="alert">
          {errorMessage}
        </p>
      ) : null}
    </>
  );
}
