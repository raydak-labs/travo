import { useState } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { Clock, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useTimezone, useSetTimezone } from '@/hooks/use-system';
import { findTimezone } from '@/lib/timezones';

const SESSION_KEY = 'timezone-alert-dismissed';

export function TimezoneAlert() {
  const { data: deviceTz } = useTimezone();
  const setTimezoneMutation = useSetTimezone();
  const navigate = useNavigate();
  const [dismissed, setDismissed] = useState(() => sessionStorage.getItem(SESSION_KEY) === 'true');

  if (dismissed || !deviceTz) return null;

  const browserTz = Intl.DateTimeFormat().resolvedOptions().timeZone;
  if (browserTz === deviceTz.zonename) return null;

  const handleDismiss = () => {
    sessionStorage.setItem(SESSION_KEY, 'true');
    setDismissed(true);
  };

  const handleUpdate = () => {
    const match = findTimezone(browserTz);
    if (match) {
      setTimezoneMutation.mutate(
        { zonename: match.zonename, timezone: match.timezone },
        {
          onSuccess: () => {
            sessionStorage.setItem(SESSION_KEY, 'true');
            setDismissed(true);
          },
        },
      );
    } else {
      // Browser timezone not in our known list — navigate to system page for manual selection
      void navigate({ to: '/system' });
    }
  };

  return (
    <div
      role="alert"
      className="flex flex-wrap items-start gap-3 rounded-lg border border-[var(--status-warn-border)] bg-[var(--status-warn-surface)] p-3 text-sm text-[var(--status-warn-text)]"
    >
      <Clock className="mt-0.5 h-4 w-4 shrink-0" />
      <span className="min-w-0 flex-1 basis-[12rem]">
        Device timezone (<strong>{deviceTz.zonename}</strong>) doesn&apos;t match your browser (
        <strong>{browserTz}</strong>).
      </span>
      <div className="ml-auto flex shrink-0 items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          className="border-[var(--status-warn-border)] text-[var(--status-warn-text)] hover:bg-[var(--status-warn-surface)]"
          onClick={handleUpdate}
          disabled={setTimezoneMutation.isPending}
        >
          {setTimezoneMutation.isPending ? 'Updating…' : 'Update'}
        </Button>
        <button
          type="button"
          aria-label="Dismiss timezone alert"
          className="rounded p-1 hover:bg-[var(--status-warn-border)]"
          onClick={handleDismiss}
        >
          <X className="h-4 w-4" />
        </button>
      </div>
    </div>
  );
}
