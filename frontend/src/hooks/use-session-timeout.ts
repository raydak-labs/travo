import { useEffect, useRef } from 'react';
import { toast } from 'sonner';
import { getToken, handleUnauthorized } from '@/lib/api-client';

const WARNING_THRESHOLD_MS = 5 * 60 * 1000; // 5 minutes
const CHECK_INTERVAL_MS = 30_000; // 30 seconds

/** Decode the exp claim from a JWT without any library. */
export function getTokenExpiry(token: string): number | null {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) return null;
    // base64url → base64, then decode
    const base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/');
    const payload = JSON.parse(atob(base64)) as { exp?: number };
    return typeof payload.exp === 'number' ? payload.exp : null;
  } catch {
    return null;
  }
}

/**
 * Monitors the JWT session and warns when it's about to expire.
 * Shows a toast warning at 5 minutes before expiry and
 * redirects to login when the token expires.
 */
export function useSessionTimeout(): void {
  const warnedRef = useRef(false);

  useEffect(() => {
    function check() {
      const token = getToken();
      if (!token) return;

      const exp = getTokenExpiry(token);
      if (exp === null) return;

      const remainingMs = exp * 1000 - Date.now();

      if (remainingMs <= 0) {
        handleUnauthorized();
        return;
      }

      if (remainingMs <= WARNING_THRESHOLD_MS && !warnedRef.current) {
        warnedRef.current = true;
        const minutes = Math.ceil(remainingMs / 60_000);
        toast.warning('Session expiring soon', {
          description: `Your session will expire in ${minutes} minute${minutes === 1 ? '' : 's'}. Please log in again to continue.`,
          duration: 10_000,
        });
      }
    }

    check();
    const id = setInterval(check, CHECK_INTERVAL_MS);
    return () => clearInterval(id);
  }, []);
}
