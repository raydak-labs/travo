import type { UseFormRegister } from 'react-hook-form';
import { AlertCircle, Loader2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import type { LoginFormValues } from '@/pages/login/login-schema';

type LoginFormFieldsProps = {
  register: UseFormRegister<LoginFormValues>;
  passwordMessage: string | undefined;
  rootMessage: string | undefined;
  isSubmitting: boolean;
};

export function LoginFormFields({
  register,
  passwordMessage,
  rootMessage,
  isSubmitting,
}: LoginFormFieldsProps) {
  return (
    <>
      <div className="space-y-2">
        <label htmlFor="password" className="text-sm font-medium text-gray-700 dark:text-gray-300">
          Password
        </label>
        <Input
          id="password"
          type="password"
          placeholder="Enter your password"
          autoFocus
          aria-invalid={!!passwordMessage}
          aria-describedby={passwordMessage ? 'password-error' : undefined}
          {...register('password')}
        />
        {passwordMessage ? (
          <p id="password-error" className="text-sm text-red-600 dark:text-red-400" role="alert">
            {passwordMessage}
          </p>
        ) : null}
      </div>

      {rootMessage ? (
        <div
          className="flex items-center gap-2 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-950/50 dark:text-red-400"
          role="alert"
        >
          <AlertCircle className="h-4 w-4 shrink-0" />
          <span>{rootMessage}</span>
        </div>
      ) : null}

      <div className="flex items-center gap-2">
        <input
          type="checkbox"
          id="remember-me"
          className="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500"
          {...register('rememberMe')}
        />
        <label htmlFor="remember-me" className="text-sm text-gray-600 dark:text-gray-400">
          Remember me
        </label>
      </div>

      <Button type="submit" className="w-full" disabled={isSubmitting}>
        {isSubmitting ? (
          <>
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            Signing in…
          </>
        ) : (
          'Sign In'
        )}
      </Button>
    </>
  );
}
