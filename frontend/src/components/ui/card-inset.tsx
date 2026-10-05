// CardInset — nested region inside Card (no second shadow)
// variants: default = border only; muted = border + bg-gray-50 dark:bg-gray-900/50
import { type HTMLAttributes, forwardRef } from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/cn';

/** Base inset chrome, exported so an ad-hoc muted nest matches the token. */
export const insetClassName = 'rounded-md border border-gray-200 p-3 dark:border-white/10';

/** The muted fill, which reads `dark:bg-gray-900/50` — not `dark:bg-gray-900`. */
export const insetMutedClassName = 'bg-gray-50 dark:bg-gray-900/50';

const cardInsetVariants = cva(insetClassName, {
  variants: {
    variant: {
      default: '',
      muted: insetMutedClassName,
    },
  },
  defaultVariants: {
    variant: 'default',
  },
});

export interface CardInsetProps
  extends HTMLAttributes<HTMLDivElement>, VariantProps<typeof cardInsetVariants> {}

const CardInset = forwardRef<HTMLDivElement, CardInsetProps>(
  ({ className, variant, ...props }, ref) => (
    <div ref={ref} className={cn(cardInsetVariants({ variant }), className)} {...props} />
  ),
);
CardInset.displayName = 'CardInset';

export { CardInset, cardInsetVariants };
