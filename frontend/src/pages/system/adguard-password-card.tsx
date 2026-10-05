import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { KeyRound } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { FieldError } from '@/components/ui/field-error';
import { Input } from '@/components/ui/input';
import { QueryCard } from '@/components/ui/query-card';
import { Skeleton } from '@/components/ui/skeleton';
import { useChangeAdGuardPassword } from '@/hooks/use-services';
import { useServices } from '@/hooks/use-services';
import {
  changeAdGuardPasswordSchema,
  type ChangeAdGuardPasswordFormValues,
} from '@/lib/schemas/system-forms';

export function AdGuardPasswordCard() {
  const servicesQuery = useServices();
  const services = servicesQuery.data ?? [];
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

  // Only a service list that arrived may hide the card; a failed GET used to
  // read as "AdGuard Home is not installed".
  if (!servicesQuery.isLoading && !servicesQuery.isError && !adguardInstalled) return null;

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
        <CardTitle>AdGuard Password</CardTitle>
        <KeyRound className="h-4 w-4 text-gray-500 dark:text-gray-400" />
      </CardHeader>

      <CardContent>
        <QueryCard
          isLoading={servicesQuery.isLoading}
          isError={servicesQuery.isError}
          error={servicesQuery.error}
          onRetry={() => void servicesQuery.refetch()}
          loading={<Skeleton className="h-8 w-full" />}
        >
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
                <FieldError id="ag-pw-new-err">{errors.new_password.message}</FieldError>
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
                <FieldError id="ag-pw-confirm-err">{errors.confirm_password.message}</FieldError>
              ) : null}
              <div className="flex flex-wrap items-center gap-2">
                <Button type="submit" disabled={changePasswordMutation.isPending || isSubmitting}>
                  {changePasswordMutation.isPending ? 'Saving…' : 'Save Password'}
                </Button>
                <Button type="button" variant="outline" onClick={resetAndClose}>
                  Cancel
                </Button>
              </div>
            </form>
          )}
        </QueryCard>
      </CardContent>
    </Card>
  );
}
