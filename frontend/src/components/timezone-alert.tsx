import { useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { Clock, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useTimezone } from '@/hooks/use-system';

const SESSION_KEY = 'timezone-alert-dismissed';

export function TimezoneAlert() {
  const { data: deviceTz } = useTimezone();
  const navigate = useNavigate();
  const [dismissed, setDismissed] = useState(() => sessionStorage.getItem(SESSION_KEY) === 'true');

  if (dismissed || !deviceTz) return null;

  const browserTz = Intl.DateTimeFormat().resolvedOptions().timeZone;
  if (browserTz === deviceTz.zonename) return null;

  const handleDismiss = () => {
    sessionStorage.setItem(SESSION_KEY, 'true');
    setDismissed(true);
  };

  return (
    <div
      role="alert"
      className="flex items-center gap-3 rounded-lg border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900 dark:border-amber-700 dark:bg-amber-950 dark:text-amber-200"
    >
      <Clock className="h-4 w-4 shrink-0" />
      <span className="flex-1">
        Device timezone (<strong>{deviceTz.zonename}</strong>) doesn&apos;t match your browser (
        <strong>{browserTz}</strong>).
      </span>
      <Button
        variant="outline"
        size="sm"
        className="border-amber-400 text-amber-900 hover:bg-amber-100 dark:border-amber-600 dark:text-amber-200 dark:hover:bg-amber-900"
        onClick={() => navigate({ to: '/system' })}
      >
        Update
      </Button>
      <button
        type="button"
        aria-label="Dismiss timezone alert"
        className="rounded p-1 hover:bg-amber-200 dark:hover:bg-amber-800"
        onClick={handleDismiss}
      >
        <X className="h-4 w-4" />
      </button>
    </div>
  );
}
