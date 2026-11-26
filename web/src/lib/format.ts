import { m } from './paraglide/messages.js';

/** Two largest non-zero units of d/h/m: 90061 -> "1d 1h"; under a minute -> "<1m". */
export function formatUptime(seconds: number): string {
  const total = Math.max(0, Math.floor(seconds));
  if (total < 60) return '<1m';
  const parts = [
    [Math.floor(total / 86400), 'd'],
    [Math.floor((total % 86400) / 3600), 'h'],
    [Math.floor((total % 3600) / 60), 'm'],
  ] as const;
  return parts
    .filter(([n]) => n > 0)
    .slice(0, 2)
    .map(([n, unit]) => `${n}${unit}`)
    .join(' ');
}

/** Binary (1024) units with SI-style labels: 428646400 -> "408.8 MB". */
export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  const units = ['KB', 'MB', 'GB'];
  let value = n / 1024;
  let i = 0;
  while (value >= 1024 && i < units.length - 1) {
    value /= 1024;
    i++;
  }
  return `${value.toFixed(1)} ${units[i]}`;
}

/** Load averages as shown to people: "0.39 · 0.24 · 0.10". */
export function formatLoad(load: readonly number[]): string {
  return load.map((l) => l.toFixed(2)).join(' · ');
}

const BIT_UNITS = ['bit/s', 'kbit/s', 'Mbit/s', 'Gbit/s'] as const;
const round = (value: number) => (value < 10 ? Math.round(value * 10) / 10 : Math.round(value));

/**
 * A link rate in SI bit units with at most two integer digits: 850 -> "0.9 kbit/s",
 * 12_000_000 -> "12 Mbit/s". One decimal below 10. Null, negative or non-finite is unknown.
 */
export function formatBitRate(bits: number | null): string {
  if (bits === null || !Number.isFinite(bits) || bits < 0) return m.dashboard_rate_unknown();
  let unit = 0;
  let value = bits;
  // Smallest unit that keeps the rounded value under 100; Gbit/s is the cap.
  while (unit < BIT_UNITS.length - 1 && round(value) >= 100) {
    value /= 1000;
    unit++;
  }
  return `${round(value)} ${BIT_UNITS[unit]}`;
}
