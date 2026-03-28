import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { KeyRound } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { useChangePassword } from '@/hooks/use-system';
import {
  changeAdminPasswordSchema,
  type ChangeAdminPasswordFormValues,
} from '@/lib/schemas/system-forms';
import { ChangePasswordCardSummary } from '@/pages/system/change-password-card-summary';
import { ChangePasswordEditFields } from '@/pages/system/change-password-edit-fields';

export function ChangePasswordCard() {
  const changePasswordMutation = useChangePassword();
  const [isEditing, setIsEditing] = useState(false);

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<ChangeAdminPasswordFormValues>({
    resolver: zodResolver(changeAdminPasswordSchema),
    defaultValues: {
      current_password: '',
      new_password: '',
      confirm_password: '',
    },
    mode: 'onChange',
  });

  const resetAndClose = () => {
    reset({
      current_password: '',
      new_password: '',
      confirm_password: '',
    });
    setIsEditing(false);
  };

  const onSubmit = (data: ChangeAdminPasswordFormValues) => {
    changePasswordMutation.mutate(
      { current_password: data.current_password, new_password: data.new_password },
      {
        onSuccess: () => {
          resetAndClose();
        },
      },
    );
  };

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">Change Password</CardTitle>
        <KeyRound className="h-4 w-4 text-gray-500" />
      </CardHeader>

      <CardContent>
        {!isEditing ? (
          <ChangePasswordCardSummary
            onEdit={() => {
              reset({
                current_password: '',
                new_password: '',
                confirm_password: '',
              });
              setIsEditing(true);
            }}
          />
        ) : (
          <form onSubmit={handleSubmit(onSubmit)} className="space-y-3" noValidate>
            <ChangePasswordEditFields register={register} errors={errors} />
            <div className="flex flex-wrap items-center gap-2">
              <Button
                type="submit"
                size="sm"
                disabled={changePasswordMutation.isPending || isSubmitting}
              >
                {changePasswordMutation.isPending ? 'Changing…' : 'Change Password'}
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
