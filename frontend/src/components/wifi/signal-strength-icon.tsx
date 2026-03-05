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
    return <WifiOff className={clsx('h-5 w-5 text-gray-400', className)} aria-label="No signal" />;
  }

  const colorMap: Record<number, string> = {
    1: 'text-red-500',
    2: 'text-yellow-500',
    3: 'text-green-500',
    4: 'text-green-600',
  };

  return (
    <div className={clsx('relative', className)} aria-label={`Signal strength ${bars} of 4 bars`}>
      <Wifi className={clsx('h-5 w-5', colorMap[bars])} />
    </div>
  );
}
