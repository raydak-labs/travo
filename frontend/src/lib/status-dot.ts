/**
 * Status dot for "this link is up" / "this link is down".
 *
 * The glow was an inline `rgba()` literal duplicated in three files — the only
 * hardcoded `rgb()` left outside `index.css`. One definition keeps the up and
 * down treatments from drifting apart again.
 */
export function statusDotClass(up: boolean): string {
  return up
    ? 'bg-emerald-500 shadow-[0_0_6px_rgba(16,185,129,0.6)] dark:bg-emerald-400'
    : 'bg-red-500 shadow-[0_0_6px_rgba(239,68,68,0.6)] dark:bg-red-400';
}

/** Neutral dot for a link that is simply not present or not applicable. */
export const statusDotIdleClass = 'bg-gray-300 dark:bg-gray-600';
