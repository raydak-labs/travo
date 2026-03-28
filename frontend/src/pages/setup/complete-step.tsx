import { CheckCircle2, Loader2 } from 'lucide-react';
import { Button } from '@/components/ui/button';

export function CompleteStep({
  onFinish,
  isPending,
}: {
  onFinish: () => void;
  isPending: boolean;
}) {
  return (
    <div className="space-y-6 text-center">
      <div className="mx-auto flex h-20 w-20 items-center justify-center rounded-2xl bg-gradient-to-br from-green-500 to-green-600 shadow-lg">
        <CheckCircle2 className="h-10 w-10 text-white" />
      </div>
      <div>
        <h2 className="text-2xl font-bold text-gray-900 dark:text-white">Setup Complete!</h2>
        <p className="mt-3 text-gray-600 dark:text-gray-400">
          Your travel router is configured and ready to use. You can adjust all settings later from
          the dashboard.
        </p>
      </div>
      <div className="rounded-lg bg-gray-50 p-4 text-left text-sm dark:bg-gray-900">
        <h3 className="mb-2 font-medium text-gray-900 dark:text-white">What&apos;s next?</h3>
        <ul className="space-y-1 text-gray-600 dark:text-gray-400">
          <li>• Monitor your connection from the Dashboard</li>
          <li>• Set up a VPN for secure browsing</li>
          <li>• Install additional services (AdGuard, etc.)</li>
          <li>• Configure advanced network settings</li>
        </ul>
      </div>
      <Button onClick={onFinish} size="lg" className="w-full" disabled={isPending}>
        {isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
        Go to Dashboard
      </Button>
    </div>
  );
}
