import { type InputHTMLAttributes, forwardRef } from 'react';
import { cn } from '@/lib/cn';

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
}

const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className, label, id, type, ...props }, ref) => (
    <div className="w-full">
      {label && (
        <label
          htmlFor={id}
          className="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
        >
          {label}
        </label>
      )}
      <input
        ref={ref}
        type={type}
        id={id}
        className={cn(
          'flex h-10 w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent disabled:cursor-not-allowed disabled:opacity-50 dark:border-gray-700 dark:bg-gray-900 dark:text-white dark:placeholder:text-gray-500',
          'file:border-0 file:bg-transparent file:text-sm file:font-medium',
          'aria-[invalid=true]:border-red-500 aria-[invalid=true]:ring-red-500',
          className,
        )}
        {...props}
      />
    </div>
  ),
);
Input.displayName = 'Input';

export { Input, type InputProps };
