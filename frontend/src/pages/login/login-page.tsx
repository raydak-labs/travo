import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useNavigate } from '@tanstack/react-router';
import { AlertTriangle } from 'lucide-react';
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
      await navigate({ to: '/dashboard-2' });
    } catch (err) {
      setError('root', {
        message: err instanceof Error ? err.message : 'Login failed',
      });
    }
  });

  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-blue-50 via-white to-blue-100 p-4 dark:from-gray-950 dark:via-gray-900 dark:to-gray-950">
      <div className="w-full max-w-md space-y-4">
        {/* SSH Warning Banner */}
        <div className="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-200">
          <div className="flex items-start gap-2">
            <AlertTriangle className="mt-0.5 h-4 w-4 flex-shrink-0" />
            <div>
              <p className="font-medium">⚠️ SSH Access Warning</p>
              <p className="mt-1">
                Manual UCI configuration changes via SSH bypass the web interface's safety mechanisms.
                This can cause device lockouts or configuration corruption. Use the web interface for
                configuration changes whenever possible.
              </p>
            </div>
          </div>
        </div>

        <Card className="shadow-lg">
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
    </div>
  );
}
