import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { KeyRound } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useChangeAdGuardPassword } from '@/hooks/use-services';
import { useServices } from '@/hooks/use-services';
import {
  changeAdGuardPasswordSchema,
  type ChangeAdGuardPasswordFormValues,
} from '@/lib/schemas/system-forms';

export function AdGuardPasswordCard() {
  const { data: services = [] } = useServices();
  const adguardInstalled = services.some(
    (s) => s.id === 'adguardhome' && s.state !== 'not_installed',
  );

  const changePasswordMutation = useChangeAdGuardPassword();
  const [isEditing, setIsEditing] = useState(false);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<ChangeAdGuardPasswordFormValues>({
    resolver: zodResolver(changeAdGuardPasswordSchema),
    defaultValues: { new_password: '', confirm_password: '' },
    mode: 'onChange',
  });

  if (!adguardInstalled) return null;

  const resetAndClose = () => {
    reset({ new_password: '', confirm_password: '' });
    setIsEditing(false);
  };

  const onSubmit = (data: ChangeAdGuardPasswordFormValues) => {
    changePasswordMutation.mutate({ password: data.new_password }, { onSuccess: resetAndClose });
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">AdGuard Password</CardTitle>
        <KeyRound className="h-4 w-4 text-gray-500" />
      </CardHeader>

      <CardContent>
        {!isEditing ? (
          <div className="flex items-center justify-between">
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Change the AdGuard Home admin password.
            </p>
            <Button size="sm" variant="outline" onClick={() => setIsEditing(true)}>
              Change
            </Button>
          </div>
        ) : (
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-3" noValidate>
            <Input
              type="password"
              placeholder="New password (min 6 characters)"
              autoComplete="new-password"
              aria-invalid={!!errors.new_password}
              aria-describedby={errors.new_password ? 'ag-pw-new-err' : undefined}
              {...register('new_password')}
            />
            {errors.new_password ? (
              <p id="ag-pw-new-err" className="text-sm text-red-500" role="alert">
                {errors.new_password.message}
              </p>
            ) : null}
            <Input
              type="password"
              placeholder="Confirm new password"
              autoComplete="new-password"
              aria-invalid={!!errors.confirm_password}
              aria-describedby={errors.confirm_password ? 'ag-pw-confirm-err' : undefined}
              {...register('confirm_password')}
            />
            {errors.confirm_password ? (
              <p id="ag-pw-confirm-err" className="text-sm text-red-500" role="alert">
                {errors.confirm_password.message}
              </p>
            ) : null}
            <div className="flex flex-wrap items-center gap-2">
              <Button
                type="submit"
                size="sm"
                disabled={changePasswordMutation.isPending || isSubmitting}
              >
                {changePasswordMutation.isPending ? 'Saving…' : 'Save Password'}
              </Button>
              <Button type="button" size="sm" variant="outline" onClick={resetAndClose}>
                Cancel
              </Button>
            </div>
          </form>
        )}
      </CardContent>
    </Card>
  );
}
