import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useNavigate } from '@tanstack/react-router';
import { useAuthStore } from '@/stores/auth-store';
import { Card, CardContent } from '@/components/ui/card';
import { loginFormSchema, type LoginFormValues } from '@/pages/login/login-schema';
import { LoginFormFields } from '@/pages/login/login-form-fields';
import { LoginPageCardHeader } from '@/pages/login/login-page-card-header';

export function LoginPage() {
  const login = useAuthStore((s) => s.login);
  const navigate = useNavigate();

  const {
    register,
    handleSubmit,
    setError,
    clearErrors,
    formState: { errors, isSubmitting },
  } = useForm<LoginFormValues>({
    resolver: zodResolver(loginFormSchema),
    defaultValues: { password: '', rememberMe: true },
  });

  const onSubmit = handleSubmit(async (data) => {
    clearErrors('root');
    try {
      await login(data.password, data.rememberMe);
      await navigate({ to: '/dashboard' });
    } catch (err) {
      setError('root', {
        message: err instanceof Error ? err.message : 'Login failed',
      });
    }
  });

  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-blue-50 via-white to-blue-100 p-4 dark:from-gray-950 dark:via-gray-900 dark:to-gray-950">
      <Card className="w-full max-w-md shadow-lg">
        <LoginPageCardHeader />
        <CardContent className="pt-2">
          <form onSubmit={onSubmit} className="space-y-5" noValidate>
            <LoginFormFields
              register={register}
              passwordMessage={errors.password?.message}
              rootMessage={errors.root?.message}
              isSubmitting={isSubmitting}
            />
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
