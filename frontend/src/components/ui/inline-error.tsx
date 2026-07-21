import { type HTMLAttributes, forwardRef } from 'react';
import { cn } from '@/lib/cn';

const InlineError = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div
      ref={ref}
      role="alert"
      className={cn(
        'rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300',
        className,
      )}
      {...props}
    />
  ),
);
InlineError.displayName = 'InlineError';

export { InlineError };
