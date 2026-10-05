import { type HTMLAttributes, forwardRef } from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/cn';

/**
 * The one way a status colour reaches the screen. Tones map to the semantic
 * tokens in `index.css`, so a status keeps its light and dark variant without
 * every call site remembering both.
 *
 * `stale` is deliberately not another colour: it means "we had a value and it
 * may no longer be true", which is weaker than `warn` and must not look like
 * an active problem.
 */
const statusPillVariants = cva('inline-flex items-center gap-1.5 rounded-full border font-medium', {
  variants: {
    tone: {
      ok: 'border-[var(--status-ok-border)] bg-[var(--status-ok-surface)] text-[var(--status-ok-text)]',
      warn: 'border-[var(--status-warn-border)] bg-[var(--status-warn-surface)] text-[var(--status-warn-text)]',
      danger:
        'border-[var(--status-danger-border)] bg-[var(--status-danger-surface)] text-[var(--status-danger-text)]',
      info: 'border-[var(--status-info-border)] bg-[var(--status-info-surface)] text-[var(--status-info-text)]',
      neutral:
        'border-[var(--status-neutral-border)] bg-[var(--status-neutral-surface)] text-[var(--status-neutral-text)]',
      stale:
        'border-[var(--status-stale-border)] bg-[var(--status-stale-surface)] text-[var(--status-stale-text)]',
    },
    size: {
      sm: 'px-2 py-0.5 text-xs',
      md: 'px-2.5 py-1 text-sm',
    },
  },
  defaultVariants: { tone: 'neutral', size: 'sm' },
});

/** Dot colour for the same tones, for rows and tables where a pill is too loud. */
export const statusDotToneClass: Record<NonNullable<StatusPillProps['tone']>, string> = {
  ok: 'bg-[var(--status-ok-border)]',
  warn: 'bg-[var(--status-warn-border)]',
  danger: 'bg-[var(--status-danger-border)]',
  info: 'bg-[var(--status-info-border)]',
  neutral: 'bg-[var(--status-neutral-border)]',
  stale: 'bg-[var(--status-stale-border)]',
};

interface StatusPillProps
  extends HTMLAttributes<HTMLSpanElement>,
    VariantProps<typeof statusPillVariants> {
  /** Renders a leading dot. Only meaningful for a pill that names its own state. */
  withDot?: boolean;
}

const StatusPill = forwardRef<HTMLSpanElement, StatusPillProps>(
  ({ className, tone, size, withDot, children, ...props }, ref) => (
    <span ref={ref} className={cn(statusPillVariants({ tone, size }), className)} {...props}>
      {withDot && tone ? (
        <span
          className={cn('h-1.5 w-1.5 shrink-0 rounded-full', statusDotToneClass[tone])}
          aria-hidden
        />
      ) : null}
      {children}
    </span>
  ),
);
StatusPill.displayName = 'StatusPill';

export { StatusPill };
export type { StatusPillProps };