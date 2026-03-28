import type { FieldErrors, UseFormRegister } from 'react-hook-form';
import { Input } from '@/components/ui/input';
import type { ChangeAdminPasswordFormValues } from '@/lib/schemas/system-forms';

type ChangePasswordEditFieldsProps = {
  register: UseFormRegister<ChangeAdminPasswordFormValues>;
  errors: FieldErrors<ChangeAdminPasswordFormValues>;
};

export function ChangePasswordEditFields({ register, errors }: ChangePasswordEditFieldsProps) {
  return (
    <>
      <Input
        type="password"
        placeholder="Current password"
        autoComplete="current-password"
        aria-invalid={!!errors.current_password}
        aria-describedby={errors.current_password ? 'sys-cpw-current-err' : undefined}
        {...register('current_password')}
      />
      {errors.current_password ? (
        <p id="sys-cpw-current-err" className="text-sm text-red-500" role="alert">
          {errors.current_password.message}
        </p>
      ) : null}
      <Input
        type="password"
        placeholder="New password (min 6 characters)"
        autoComplete="new-password"
        aria-invalid={!!errors.new_password}
        aria-describedby={errors.new_password ? 'sys-cpw-new-err' : undefined}
        {...register('new_password')}
      />
      {errors.new_password ? (
        <p id="sys-cpw-new-err" className="text-sm text-red-500" role="alert">
          {errors.new_password.message}
        </p>
      ) : null}
      <Input
        type="password"
        placeholder="Confirm new password"
        autoComplete="new-password"
        aria-invalid={!!errors.confirm_password}
        aria-describedby={errors.confirm_password ? 'sys-cpw-confirm-err' : undefined}
        {...register('confirm_password')}
      />
      {errors.confirm_password ? (
        <p id="sys-cpw-confirm-err" className="text-sm text-red-500" role="alert">
          {errors.confirm_password.message}
        </p>
      ) : null}
    </>
  );
}
