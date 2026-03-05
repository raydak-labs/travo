import { type HTMLAttributes, forwardRef } from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/cn';

const badgeVariants = cva(
  'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium',
  {
    variants: {
      variant: {
        default: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200',
        success: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200',
        warning: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200',
        destructive: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200',
        outline: 'border border-gray-300 text-gray-700 dark:border-gray-600 dark:text-gray-300',
        secondary: 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-200',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  },
);

interface BadgeProps extends HTMLAttributes<HTMLSpanElement>, VariantProps<typeof badgeVariants> {}

const Badge = forwardRef<HTMLSpanElement, BadgeProps>(({ className, variant, ...props }, ref) => (
  <span ref={ref} className={cn(badgeVariants({ variant, className }))} {...props} />
));
Badge.displayName = 'Badge';

export { Badge, badgeVariants, type BadgeProps };
