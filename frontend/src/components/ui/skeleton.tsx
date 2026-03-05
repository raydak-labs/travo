import { type HTMLAttributes, forwardRef } from 'react';
import { cn } from '@/lib/cn';

const Skeleton = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div
      ref={ref}
      className={cn('animate-pulse rounded-md bg-gray-200 dark:bg-gray-700', className)}
      {...props}
    />
  ),
);
Skeleton.displayName = 'Skeleton';

export { Skeleton };
