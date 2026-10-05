import { type TextareaHTMLAttributes, forwardRef } from 'react';
import { cn } from '@/lib/cn';
import { fieldClassName, invalidFieldClassName } from '@/components/ui/input';

/**
 * The `Input` counterpart for multi-line fields.
 *
 * Textareas were fully hand-rolled, and one of them (the WireGuard profile
 * importer) had no focus ring at all.
 */
const Textarea = forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement>>(
  ({ className, ...props }, ref) => (
    <textarea
      ref={ref}
      className={cn(fieldClassName, invalidFieldClassName, 'min-h-24 font-mono', className)}
      {...props}
    />
  ),
);
Textarea.displayName = 'Textarea';

export { Textarea };
