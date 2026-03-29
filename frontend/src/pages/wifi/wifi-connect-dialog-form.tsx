import { Eye, EyeOff, Wifi } from 'lucide-react';
import type { FieldErrors, UseFormHandleSubmit, UseFormRegister } from 'react-hook-form';
import type { GroupedScanNetwork } from '@shared/index';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { SecurityBadge } from '@/components/wifi/security-badge';
import { formatWifiBandLabel } from '@/lib/wifi-band';
import type { WifiConnectFormValues } from '@/lib/schemas/wifi-forms';
import { signalQuality, type BandOption } from './wifi-connect-utils';

type WifiConnectDialogFormProps = {
  group: GroupedScanNetwork;
  bandOptions: BandOption[];
  needsPassword: boolean;
  register: UseFormRegister<WifiConnectFormValues>;
  handleSubmit: UseFormHandleSubmit<WifiConnectFormValues>;
  onValidSubmit: (data: WifiConnectFormValues) => void;
  errors: FieldErrors<WifiConnectFormValues>;
  passwordValue: string;
  showPassword: boolean;
  setShowPassword: (next: boolean) => void;
  error: string | null;
  isConnecting: boolean;
  onCancel: () => void;
};

export function WifiConnectDialogForm({
  group,
  bandOptions,
  needsPassword,
  register,
  handleSubmit,
  onValidSubmit,
  errors,
  passwordValue,
  showPassword,
  setShowPassword,
  error,
  isConnecting,
  onCancel,
}: WifiConnectDialogFormProps) {
  return (
    <div>
      <div className="mb-4 flex items-center gap-3">
        <Wifi className="h-5 w-5 text-blue-500" />
        <div>
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">{group.ssid}</h2>
          <SecurityBadge encryption={group.encryption} />
        </div>
      </div>

      <form onSubmit={handleSubmit(onValidSubmit)} className="space-y-4" noValidate>
        {bandOptions.length > 1 && (
          <div className="space-y-2">
            <p className="text-sm font-medium text-gray-700 dark:text-gray-300">Band</p>
            <div className="space-y-2">
              {bandOptions.map(({ band, dbm }) => (
                <label
                  key={band}
                  className="flex cursor-pointer items-center gap-2 rounded border p-2 has-[:checked]:border-blue-500 has-[:checked]:bg-blue-50 dark:has-[:checked]:bg-blue-950/30"
                >
                  <input
                    type="radio"
                    className="h-4 w-4"
                    value={band}
                    {...register('selectedBand')}
                  />
                  <span className="text-sm text-gray-900 dark:text-white">
                    {formatWifiBandLabel(band)} ({dbm} dBm, {signalQuality(dbm)})
                  </span>
                </label>
              ))}
            </div>
          </div>
        )}

        {needsPassword && (
          <div className="relative">
            <Input
              id="wifi-password"
              label="Password"
              type={showPassword ? 'text' : 'password'}
              autoFocus={bandOptions.length <= 1}
              aria-required="true"
              aria-invalid={errors.password ? 'true' : undefined}
              aria-describedby={errors.password ? 'password-error' : undefined}
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
                id="password-error"
                className="mt-1 text-sm text-red-600 dark:text-red-400"
                role="alert"
              >
                {errors.password.message}
              </p>
            ) : null}
          </div>
        )}

        {error ? (
          <p className="text-sm text-red-600 dark:text-red-400" role="alert">
            {error}
          </p>
        ) : null}

        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" onClick={onCancel} disabled={isConnecting}>
            Cancel
          </Button>
          <Button type="submit" disabled={isConnecting || (needsPassword && !passwordValue)}>
            {isConnecting ? 'Connecting...' : 'Connect'}
          </Button>
        </div>
      </form>
    </div>
  );
}
