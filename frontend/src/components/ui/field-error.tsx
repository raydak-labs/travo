import { type HTMLAttributes, forwardRef } from 'react';
import { cn } from '@/lib/cn';

/**
 * Validation error text, in one treatment.
 *
 * Three treatments were in use (`text-xs text-red-500`, `text-sm text-red-500`,
 * `text-sm text-red-600 dark:text-red-400`), so the same failure read at two
 * sizes and two hues depending on the form. `red-500` on white is 3.76:1 and
 * fails AA for normal-size text — and this is exactly the text a user must be
 * able to read. The danger tone carries its own light and dark pair (ADR 0012).
 */
const FieldError = forwardRef<HTMLParagraphElement, HTMLAttributes<HTMLParagraphElement>>(
  ({ className, ...props }, ref) => (
    <p
      ref={ref}
      role="alert"
      className={cn('mt-1 text-xs text-[var(--status-danger-text)]', className)}
      {...props}
    />
  ),
);
FieldError.displayName = 'FieldError';

export { FieldError };
