import { useCallback, useState } from 'react';
import { isWifiLockoutError } from '@/lib/wifi-lockout';

/**
 * Drives the acknowledgement dialog for any mutating wireless request.
 *
 * A caller sends its request WITHOUT the acknowledgement, and passes the error
 * plus the acknowledged version of the same request to `onLockout`. When the
 * router answered with the lockout code, `open` flips to true and nothing is
 * re-sent until the operator ticks the box; any other error is left alone, so an
 * ordinary failure never shows up as a lockout.
 */
export function useWifiLockout() {
  const [retry, setRetry] = useState<(() => void) | null>(null);

  /** Returns true when the error was the lockout and the dialog is now open. */
  const onLockout = useCallback((error: unknown, resend: () => void) => {
    if (!isWifiLockoutError(error)) return false;
    setRetry(() => resend);
    return true;
  }, []);

  const acknowledge = useCallback(() => {
    const resend = retry;
    setRetry(null);
    resend?.();
  }, [retry]);

  const dismiss = useCallback(() => setRetry(null), []);

  return { open: retry !== null, onLockout, acknowledge, dismiss };
}
