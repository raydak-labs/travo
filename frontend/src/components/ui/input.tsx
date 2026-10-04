import { type InputHTMLAttributes, forwardRef } from 'react';
import { cn } from '@/lib/cn';
import { Label } from '@/components/ui/label';

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
}

/**
 * The single source of truth for form-control chrome.
 *
 * This class string was copy-pasted into five call sites and had already
 * drifted (`focus-visible:ring-1`, a different dark fill), so an Input and a
 * Select sitting in the same form could render two different surfaces.
 */
export const fieldClassName =
  'flex h-10 w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-700 dark:bg-gray-900 dark:text-white dark:placeholder:text-gray-500';

export const invalidFieldClassName =
  'aria-[invalid=true]:border-red-500 aria-[invalid=true]:ring-red-500';

const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className, label, id, type, ...props }, ref) => (
    <div className="w-full">
      {label && (
        <Label htmlFor={id} className="mb-1 block">
          {label}
        </Label>
      )}
      <input
        ref={ref}
        type={type}
        id={id}
        className={cn(
          fieldClassName,
          invalidFieldClassName,
          'file:border-0 file:bg-transparent file:text-sm file:font-medium',
          className,
        )}
        {...props}
      />
    </div>
  ),
);
Input.displayName = 'Input';

export { Input, type InputProps };
