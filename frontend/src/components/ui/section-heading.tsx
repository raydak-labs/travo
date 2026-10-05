import { type HTMLAttributes, forwardRef } from 'react';
import { cn } from '@/lib/cn';

/**
 * Group label inside a page body. Six pages had hand-rolled the same `<h2>`
 * class string; this is that string, so a future change lands in one place.
 */
const SectionHeading = forwardRef<HTMLHeadingElement, HTMLAttributes<HTMLHeadingElement>>(
  ({ className, ...props }, ref) => (
    <h2
      ref={ref}
      className={cn(
        'mb-3 text-xs font-semibold uppercase tracking-wider text-gray-400 dark:text-gray-500',
        className,
      )}
      {...props}
    />
  ),
);
SectionHeading.displayName = 'SectionHeading';

export { SectionHeading };