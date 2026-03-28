import type { FieldErrors, UseFormRegister } from 'react-hook-form';
import { Eye, EyeOff } from 'lucide-react';
import { Input } from '@/components/ui/input';
import type { SetupApFormValues } from '@/pages/setup/setup-schema';

type APStepCredentialsFieldsProps = {
  register: UseFormRegister<SetupApFormValues>;
  errors: FieldErrors<SetupApFormValues>;
  showPassword: boolean;
  onTogglePassword: () => void;
};

export function APStepCredentialsFields({
  register,
  errors,
  showPassword,
  onTogglePassword,
}: APStepCredentialsFieldsProps) {
  return (
    <>
      <div>
        <label
          htmlFor="setup-ap-ssid"
          className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          Network Name (SSID)
        </label>
        <Input
          id="setup-ap-ssid"
          aria-invalid={!!errors.ssid}
          aria-describedby={errors.ssid ? 'setup-ap-ssid-err' : undefined}
          {...register('ssid')}
          placeholder="e.g. MyTravelRouter"
        />
        {errors.ssid ? (
          <p id="setup-ap-ssid-err" className="mt-1 text-xs text-red-500" role="alert">
            {errors.ssid.message}
          </p>
        ) : null}
      </div>
      <div>
        <label
          htmlFor="setup-ap-key"
          className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          Password
        </label>
        <div className="relative">
          <Input
            id="setup-ap-key"
            type={showPassword ? 'text' : 'password'}
            autoComplete="new-password"
            aria-invalid={!!errors.key}
            aria-describedby={errors.key ? 'setup-ap-key-err' : undefined}
            {...register('key')}
            placeholder="Set AP password (min 8 chars)"
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
        {errors.key ? (
          <p id="setup-ap-key-err" className="mt-1 text-xs text-red-500" role="alert">
            {errors.key.message}
          </p>
        ) : null}
      </div>
    </>
  );
}
