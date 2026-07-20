import { type LabelHTMLAttributes, forwardRef } from 'react';
import { cn } from '@/lib/cn';

const Label = forwardRef<HTMLLabelElement, LabelHTMLAttributes<HTMLLabelElement>>(
  ({ className, ...props }, ref) => (
    <label
      ref={ref}
      className={cn('text-xs font-medium text-gray-500 dark:text-gray-400', className)}
      {...props}
    />
  ),
);
Label.displayName = 'Label';

export { Label };
