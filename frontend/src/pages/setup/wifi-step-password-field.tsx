import type { FieldErrors, UseFormRegister } from 'react-hook-form';
import { Eye, EyeOff } from 'lucide-react';
import { Input } from '@/components/ui/input';
import type { SetupWifiFormValues } from '@/pages/setup/setup-schema';

type WifiStepPasswordFieldProps = {
  selectedSsid: string;
  showPassword: boolean;
  onTogglePassword: () => void;
  register: UseFormRegister<SetupWifiFormValues>;
  errors: FieldErrors<SetupWifiFormValues>;
};

export function WifiStepPasswordField({
  selectedSsid,
  showPassword,
  onTogglePassword,
  register,
  errors,
}: WifiStepPasswordFieldProps) {
  return (
    <div>
      <label
        htmlFor="setup-wifi-psk"
        className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
      >
        Password for &quot;{selectedSsid}&quot;
      </label>
      <div className="relative">
        <Input
          id="setup-wifi-psk"
          type={showPassword ? 'text' : 'password'}
          autoComplete="off"
          aria-invalid={!!errors.wifiPassword}
          aria-describedby={errors.wifiPassword ? 'setup-wifi-psk-err' : undefined}
          {...register('wifiPassword')}
          placeholder="Enter WiFi password"
        />
        <button
          type="button"
          className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
          onClick={onTogglePassword}
          aria-label={showPassword ? 'Hide password' : 'Show password'}
        >
          {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
        </button>
      </div>
      {errors.wifiPassword ? (
        <p id="setup-wifi-psk-err" className="mt-1 text-xs text-red-500" role="alert">
          {errors.wifiPassword.message}
        </p>
      ) : null}
    </div>
  );
}
