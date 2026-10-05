import type { FieldErrors, UseFormRegister } from 'react-hook-form';
import { FieldError } from '@/components/ui/field-error';
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
        <FieldError id="sys-cpw-current-err">{errors.current_password.message}</FieldError>
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
        <FieldError id="sys-cpw-new-err">{errors.new_password.message}</FieldError>
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
        <FieldError id="sys-cpw-confirm-err">{errors.confirm_password.message}</FieldError>
      ) : null}
    </>
  );
}
