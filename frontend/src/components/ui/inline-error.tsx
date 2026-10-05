import { type HTMLAttributes, forwardRef } from 'react';
import { cn } from '@/lib/cn';

const InlineError = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div
      ref={ref}
      role="alert"
      className={cn(
        'rounded-md border border-[var(--status-danger-border)] bg-[var(--status-danger-surface)] p-3 text-sm text-[var(--status-danger-text)]',
        className,
      )}
      {...props}
    />
  ),
);
InlineError.displayName = 'InlineError';

export { InlineError };
