import type { FieldErrors, UseFormRegister } from 'react-hook-form';
import { Eye, EyeOff } from 'lucide-react';
import { Input } from '@/components/ui/input';
import type { SetupPasswordFormValues } from '@/pages/setup/setup-schema';

type PasswordStepFormFieldsProps = {
  register: UseFormRegister<SetupPasswordFormValues>;
  errors: FieldErrors<SetupPasswordFormValues>;
  showPassword: boolean;
  onTogglePassword: () => void;
};

export function PasswordStepFormFields({
  register,
  errors,
  showPassword,
  onTogglePassword,
}: PasswordStepFormFieldsProps) {
  return (
    <>
      <div>
        <label
          htmlFor="setup-current-password"
          className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          Current Password
        </label>
        <Input
          id="setup-current-password"
          type={showPassword ? 'text' : 'password'}
          autoComplete="current-password"
          aria-invalid={!!errors.current_password}
          aria-describedby={errors.current_password ? 'setup-current-password-err' : undefined}
          {...register('current_password')}
          placeholder="Enter current password"
        />
        {errors.current_password ? (
          <p id="setup-current-password-err" className="mt-1 text-xs text-red-500" role="alert">
            {errors.current_password.message}
          </p>
        ) : null}
      </div>
      <div>
        <label
          htmlFor="setup-new-password"
          className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          New Password
        </label>
        <div className="relative">
          <Input
            id="setup-new-password"
            type={showPassword ? 'text' : 'password'}
            autoComplete="new-password"
            aria-invalid={!!errors.new_password}
            aria-describedby={errors.new_password ? 'setup-new-password-err' : undefined}
            {...register('new_password')}
            placeholder="Enter new password (min 8 chars)"
          />
          <button
            type="button"
            className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
            onClick={onTogglePassword}
            aria-label={showPassword ? 'Hide passwords' : 'Show passwords'}
          >
            {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
          </button>
        </div>
        {errors.new_password ? (
          <p id="setup-new-password-err" className="mt-1 text-xs text-red-500" role="alert">
            {errors.new_password.message}
          </p>
        ) : null}
      </div>
      <div>
        <label
          htmlFor="setup-confirm-password"
          className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          Confirm New Password
        </label>
        <Input
          id="setup-confirm-password"
          type={showPassword ? 'text' : 'password'}
          autoComplete="new-password"
          aria-invalid={!!errors.confirm_password}
          aria-describedby={errors.confirm_password ? 'setup-confirm-password-err' : undefined}
          {...register('confirm_password')}
          placeholder="Confirm new password"
        />
        {errors.confirm_password ? (
          <p id="setup-confirm-password-err" className="mt-1 text-xs text-red-500" role="alert">
            {errors.confirm_password.message}
          </p>
        ) : null}
      </div>
    </>
  );
}
