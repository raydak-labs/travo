import { Wifi, WifiOff } from 'lucide-react';
import { clsx } from 'clsx';

interface SignalStrengthIconProps {
  signalPercent: number;
  className?: string;
}

function getBars(signalPercent: number): number {
  if (signalPercent >= 75) return 4;
  if (signalPercent >= 50) return 3;
  if (signalPercent >= 25) return 2;
  if (signalPercent > 0) return 1;
  return 0;
}

export function SignalStrengthIcon({ signalPercent, className }: SignalStrengthIconProps) {
  const bars = getBars(signalPercent);

  if (bars === 0) {
    return (
      <span role="img" aria-label="No signal">
        <WifiOff className={clsx('h-5 w-5 text-gray-500 dark:text-gray-400', className)} />
      </span>
    );
  }

  // Strength is a status, so it uses the status tokens rather than raw hues:
  // they carry a dark-mode variant and stay legible on both card surfaces.
  const colorMap: Record<number, string> = {
    1: 'text-[var(--status-danger-text)]',
    2: 'text-[var(--status-warn-text)]',
    3: 'text-[var(--status-ok-text)]',
    4: 'text-[var(--status-ok-text)]',
  };

  return (
    // role="img", not a bare div: aria-label is ignored on role=generic, so
    // signal quality — the deciding input on the repeater network picker —
    // reached screen readers only through icon colour.
    <span
      role="img"
      aria-label={`Signal strength ${bars} of 4 bars (${signalPercent}%)`}
      className={clsx('relative', className)}
    >
      <Wifi className={clsx('h-5 w-5', colorMap[bars])} />
    </span>
  );
}
