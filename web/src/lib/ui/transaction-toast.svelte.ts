import { onDestroy } from 'svelte';
import { toast } from 'svelte-sonner';
import TransactionToast from './TransactionToast.svelte';

/**
 * What a guarded change shows while it runs. Pages map their own state to this
 * (`joinPhase`, `controlPhase`); the toast owns the layout, countdown and buttons.
 * `deadline` is on the `performance.now()` clock.
 */
export type TxnPhase =
  | { phase: 'idle' }
  | { phase: 'applying'; text: string }
  | { phase: 'pending'; deadline: number; settling: boolean }
  | { phase: 'restoring'; text: string }
  | { phase: 'done'; tone: 'success' | 'info' | 'error'; text: string };

/** Open phases stay until they settle; errors stay until dismissed; the rest fade. */
export function toastTiming(phase: TxnPhase): { duration: number; dismissible: boolean } {
  if (phase.phase !== 'done') return { duration: Number.POSITIVE_INFINITY, dismissible: false };
  return {
    duration: phase.tone === 'error' ? Number.POSITIVE_INFINITY : 5000,
    dismissible: true,
  };
}

/**
 * Mirrors `read()` into one toast with a stable `id`. Call during component init.
 * An open toast (applying, pending, restoring) closes with its owner: its buttons
 * need the owner's request, and an unconfirmed change rolls back on its own.
 */
export function transactionToast(
  id: string,
  read: () => TxnPhase,
  settle: (confirm: boolean) => void,
) {
  let last = '';
  let open = false;
  $effect(() => {
    const phase = read();
    const key = JSON.stringify(phase);
    if (key === last) return;
    const shown = last !== '';
    last = key;
    open = phase.phase !== 'idle' && phase.phase !== 'done';
    if (phase.phase === 'idle') {
      if (shown) toast.dismiss(id);
      return;
    }
    toast.custom(TransactionToast, {
      id,
      ...toastTiming(phase),
      componentProps: { phase, settle },
    });
  });
  onDestroy(() => {
    if (open) toast.dismiss(id);
  });
}
