import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Loader2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useChangePassword } from '@/hooks/use-system';
import { PasswordStepFormFields } from '@/pages/setup/password-step-form-fields';
import { PasswordStepIntro } from '@/pages/setup/password-step-intro';
import { setupPasswordFormSchema, type SetupPasswordFormValues } from '@/pages/setup/setup-schema';

export function PasswordStep({ onNext, onBack }: { onNext: () => void; onBack: () => void }) {
  const [showPassword, setShowPassword] = useState(false);
  const changePasswordMutation = useChangePassword();

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<SetupPasswordFormValues>({
    resolver: zodResolver(setupPasswordFormSchema),
    defaultValues: {
      current_password: '',
      new_password: '',
      confirm_password: '',
    },
    mode: 'onChange',
  });

  const onSubmit = (data: SetupPasswordFormValues) => {
    changePasswordMutation.mutate(
      { current_password: data.current_password, new_password: data.new_password },
      { onSuccess: () => onNext() },
    );
  };

  return (
    <div className="space-y-6">
      <PasswordStepIntro />

      <form onSubmit={handleSubmit(onSubmit)} className="space-y-4" noValidate>
        <PasswordStepFormFields
          register={register}
          errors={errors}
          showPassword={showPassword}
          onTogglePassword={() => setShowPassword((v) => !v)}
        />

        <div className="flex gap-3">
          <Button type="button" variant="outline" onClick={onBack} className="flex-1">
            Back
          </Button>
          <Button type="submit" disabled={changePasswordMutation.isPending} className="flex-1">
            {changePasswordMutation.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            Change Password
          </Button>
        </div>
      </form>
      <button
        type="button"
        onClick={onNext}
        className="block w-full text-center text-sm text-gray-400 transition-colors hover:text-gray-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 dark:hover:text-gray-300"
      >
        Skip for now
      </button>
    </div>
  );
}
