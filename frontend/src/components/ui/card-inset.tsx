// CardInset — nested region inside Card (no second shadow)
// variants: default = border only; muted = border + bg-gray-50 dark:bg-gray-900/50
import { type HTMLAttributes, forwardRef } from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/cn';

const cardInsetVariants = cva('rounded-md border border-gray-200 p-3 dark:border-white/10', {
  variants: {
    variant: {
      default: '',
      muted: 'bg-gray-50 dark:bg-gray-900/50',
    },
  },
  defaultVariants: {
    variant: 'default',
  },
});

export interface CardInsetProps
  extends HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof cardInsetVariants> {}

const CardInset = forwardRef<HTMLDivElement, CardInsetProps>(
  ({ className, variant, ...props }, ref) => (
    <div ref={ref} className={cn(cardInsetVariants({ variant }), className)} {...props} />
  ),
);
CardInset.displayName = 'CardInset';

export { CardInset, cardInsetVariants };
