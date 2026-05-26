import { AlertTriangle } from 'lucide-react';
import type { AmneziaWGAvailability } from '@shared/index';

type Props = {
  availability: AmneziaWGAvailability | undefined;
  hasAmneziaProfiles: boolean;
};

export function AmneziaWGAvailabilityBanner({ availability, hasAmneziaProfiles }: Props) {
  if (!hasAmneziaProfiles || availability?.ready) {
    return null;
  }

  return (
    <div
      role="alert"
      className="flex gap-3 rounded-lg border border-purple-200 bg-purple-50 p-4 dark:border-purple-800 dark:bg-purple-950/30"
    >
      <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-purple-600 dark:text-purple-400" />
      <div className="space-y-1 text-sm">
        <p className="font-medium text-purple-900 dark:text-purple-100">AmneziaWG packages required</p>
        <p className="text-xs text-purple-800 dark:text-purple-200">
          You have AmneziaWG profiles but the required packages are not installed. AmneziaWG adds
          DPI-resistant obfuscation to WireGuard — ideal for restrictive networks.
          {availability?.reason && (
            <span className="ml-1 text-purple-700 dark:text-purple-300">({availability.reason})</span>
          )}
        </p>
        <p className="text-xs text-purple-700 dark:text-purple-300">
          Install from the Services page to enable AmneziaWG profiles.
        </p>
      </div>
    </div>
  );
}
