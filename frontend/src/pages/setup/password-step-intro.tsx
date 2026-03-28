import { KeyRound } from 'lucide-react';

export function PasswordStepIntro() {
  return (
    <div className="text-center">
      <KeyRound className="mx-auto h-10 w-10 text-blue-500" />
      <h2 className="mt-3 text-xl font-bold text-gray-900 dark:text-white">
        Change Default Password
      </h2>
      <p className="mt-2 text-sm text-gray-600 dark:text-gray-400">
        For security, change the default admin password before using the router.
      </p>
    </div>
  );
}
