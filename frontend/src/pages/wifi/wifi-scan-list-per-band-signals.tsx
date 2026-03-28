import type { WifiScanResult } from '@shared/index';
import { SignalStrengthIcon } from '@/components/wifi/signal-strength-icon';
import { formatWifiBandLabel, normalizeWifiBandKey } from '@/lib/wifi-band';

type WifiScanListPerBandSignalsProps = {
  aps: readonly WifiScanResult[];
};

export function WifiScanListPerBandSignals({ aps }: WifiScanListPerBandSignalsProps) {
  const byBand = new Map<string, WifiScanResult>();
  for (const ap of aps) {
    const k = normalizeWifiBandKey(ap.band);
    const existing = byBand.get(k);
    if (!existing || ap.signal_dbm > existing.signal_dbm) byBand.set(k, ap);
  }
  const bands = Array.from(byBand.entries()).sort(([a], [b]) => a.localeCompare(b));
  return (
    <div className="mt-0.5 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-gray-600 dark:text-gray-400">
      {bands.map(([k, ap]) => (
        <span key={k}>
          {formatWifiBandLabel(ap.band)}{' '}
          <SignalStrengthIcon signalPercent={ap.signal_percent} className="inline-block h-3.5 w-3.5" />{' '}
          {ap.signal_dbm} dBm
        </span>
      ))}
    </div>
  );
}
