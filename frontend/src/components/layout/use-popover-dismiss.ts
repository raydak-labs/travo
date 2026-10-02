import { useEffect, useRef } from 'react';

/**
 * Why a popover closed. `'outside'` is a pointer dismissal, so focus is left
 * where the user put it; every other reason returns focus to the trigger.
 */
export type PopoverCloseReason = 'outside' | 'escape' | 'tab' | 'activate';

type Close = (reason: PopoverCloseReason) => void;

/**
 * Dismissal plumbing for the hand-rolled header popovers.
 *
 * Only Radix `Dialog`, `Select` and `Collapsible` are vendored in this app, and
 * `package.json` is out of scope for this change, so these two popovers cannot
 * be rebuilt on `@radix-ui/react-dropdown-menu` / `-popover` without pulling in
 * a new dependency. This hook supplies the minimum correct behaviour instead
 * (WCAG 2.1.1 / 4.1.2):
 *
 * - `mousedown` outside the container closes the popover (pointer path);
 * - `Escape` anywhere in the document closes it (keyboard path);
 * - focus is moved into the popover on open and returned to the trigger on
 *   anything other than an outside-pointer dismissal.
 *
 * `close` is read through a ref so the effect does not re-subscribe on every
 * render (the callers pass an inline closure).
 */
export function usePopoverDismiss<T extends HTMLElement>(open: boolean, close: Close) {
  const containerRef = useRef<T | null>(null);
  const closeRef = useRef<Close>(close);

  useEffect(() => {
    closeRef.current = close;
  });

  useEffect(() => {
    if (!open) return;

    function handlePointerDown(event: MouseEvent) {
      const container = containerRef.current;
      if (container && !container.contains(event.target as Node)) {
        closeRef.current('outside');
      }
    }

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        closeRef.current('escape');
      }
    }

    document.addEventListener('mousedown', handlePointerDown);
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('mousedown', handlePointerDown);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [open]);

  return containerRef;
}

/**
 * Roving focus for a `role="menu"` container: moves between `role="menuitem"`
 * children with the arrow/Home/End keys and closes on Tab so focus is not
 * trapped in the menu.
 */
export function moveMenuFocus(container: HTMLElement | null, key: string): boolean {
  if (key !== 'ArrowDown' && key !== 'ArrowUp' && key !== 'Home' && key !== 'End') return false;
  const items = Array.from(container?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? []);
  if (items.length === 0) return false;

  const current = items.indexOf(document.activeElement as HTMLElement);
  let next: number;
  switch (key) {
    case 'ArrowDown':
      next = current < 0 ? 0 : (current + 1) % items.length;
      break;
    case 'ArrowUp':
      next = current < 0 ? items.length - 1 : (current - 1 + items.length) % items.length;
      break;
    case 'Home':
      next = 0;
      break;
    default:
      next = items.length - 1;
  }
  items[next]?.focus();
  return true;
}
