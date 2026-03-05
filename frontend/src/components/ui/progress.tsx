import { type HTMLAttributes, forwardRef } from 'react';
import { cn } from '@/lib/cn';

interface ProgressProps extends HTMLAttributes<HTMLDivElement> {
  value: number;
  max?: number;
}

const Progress = forwardRef<HTMLDivElement, ProgressProps>(
  ({ className, value, max = 100, ...props }, ref) => {
    const percent = Math.min(100, Math.max(0, (value / max) * 100));
    return (
      <div
        ref={ref}
        role="progressbar"
        aria-valuenow={value}
        aria-valuemin={0}
        aria-valuemax={max}
        className={cn(
          'h-2 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-gray-700',
          className,
        )}
        {...props}
      >
        <div
          className="h-full rounded-full bg-blue-600 transition-all dark:bg-blue-500"
          style={{ width: `${percent}%` }}
        />
      </div>
    );
  },
);
Progress.displayName = 'Progress';

export { Progress, type ProgressProps };
