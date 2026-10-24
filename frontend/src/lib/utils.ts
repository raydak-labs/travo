/**
 * Format bytes into a human-readable string (KB, MB, GB).
 */
export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const k = 1024;
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  const value = bytes / Math.pow(k, i);
  return `${value.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

/**
 * Format a byte-per-second rate into a human-readable string (B/s, KB/s, MB/s).
 */
export function formatRate(bytesPerSec: number): string {
  if (bytesPerSec <= 0) return '0 B/s';
  const units = ['B/s', 'KB/s', 'MB/s', 'GB/s'];
  const k = 1024;
  const i = Math.floor(Math.log(bytesPerSec) / Math.log(k));
  const value = bytesPerSec / Math.pow(k, i);
  return `${value.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

/**
 * Format a bit rate with at most two integer digits.
 *
 * Counters are sampled in bytes, but link speeds are quoted in bits. Keeping
 * the integer part to two digits is what makes a rate fit next to a topology
 * label: 850 Kbit/s reads as `0.9 Mbit/s`, and 100 Mbit/s moves to `0.1
 * Gbit/s` instead of growing a third digit. One decimal is enough for a live
 * sample that changes every two seconds.
 */
export function formatBitRate(bitsPerSec: number): string {
  if (!Number.isFinite(bitsPerSec) || bitsPerSec <= 0) return '0.0 Mbit/s';

  const mbit = bitsPerSec / 1_000_000;
  if (mbit < 0.05) return '<0.1 Mbit/s';

  const units = [
    { divisor: 1_000_000, label: 'Mbit/s' },
    { divisor: 1_000_000_000, label: 'Gbit/s' },
    { divisor: 1_000_000_000_000, label: 'Tbit/s' },
  ] as const;
  for (let index = 0; index < units.length; index += 1) {
    const unit = units[index]!;
    const value = bitsPerSec / unit.divisor;
    const rounded = Math.round(value * 10) / 10;
    if (rounded < 100 || index === units.length - 1) {
      return `${rounded.toFixed(1)} ${unit.label}`;
    }
  }
  return '0.0 Mbit/s';
}

/**
 * Format seconds into "X days, Y hours, Z minutes".
 */
export function formatUptime(seconds: number): string {
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);

  const parts: string[] = [];
  if (days > 0) parts.push(`${days} day${days !== 1 ? 's' : ''}`);
  if (hours > 0) parts.push(`${hours} hour${hours !== 1 ? 's' : ''}`);
  if (minutes > 0) parts.push(`${minutes} minute${minutes !== 1 ? 's' : ''}`);

  return parts.length > 0 ? parts.join(', ') : '0 minutes';
}
