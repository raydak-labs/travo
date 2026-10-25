import { Link } from '@tanstack/react-router';
import { ArrowUpCircle } from 'lucide-react';
import { useUpdates } from '@/hooks/use-updates';
import { summarizeUpdates } from '@/lib/updates';
import { UPDATES_ANCHOR } from '@/pages/system/system-updates-card';

/** Shown while an OpenWrt release or package updates are on offer. */
export function HeaderUpdatesIndicator() {
  const { data } = useUpdates();
  const summary = summarizeUpdates(data);
  if (!summary.any) return null;

  return (
    <Link
      to="/system"
      hash={UPDATES_ANCHOR}
      title={`Updates available: ${summary.label}`}
      aria-label={`Updates available: ${summary.label}`}
      className="inline-flex items-center gap-1.5 rounded-full border border-[var(--status-info-border)] bg-[var(--status-info-surface)] px-2.5 py-1 text-xs font-medium text-[var(--status-info-text)] transition-colors hover:opacity-90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
    >
      <ArrowUpCircle className="h-3.5 w-3.5" aria-hidden />
      <span className="hidden sm:inline">Update available</span>
    </Link>
  );
}
