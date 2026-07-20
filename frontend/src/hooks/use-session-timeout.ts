import { useEffect, useRef } from 'react';
import { toast } from 'sonner';
import { apiClient, getToken } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type { SessionResponse } from '@shared/index';

const WARNING_THRESHOLD_MS = 5 * 60 * 1000; // 5 minutes
const CHECK_INTERVAL_MS = 30_000; // 30 seconds

/**
 * Monitors the session and warns when it's about to expire.
 *
 * The remaining lifetime comes from the server as a relative duration
 * (`expires_in` seconds) and counts down against performance.now(), so no
 * wall-clock timestamps are ever compared across machines — a router or
 * client with a wrong clock can no longer cause spurious logouts.
 *
 * When the local countdown runs out the hook re-validates with the server
 * instead of logging out on its own: a real expiry surfaces as a 401, which
 * api-client already turns into a redirect to /login.
 */
export function useSessionTimeout(): void {
  const warnedRef = useRef(false);

  useEffect(() => {
    let cancelled = false;
    let refreshing = false;
    // Deadline in performance.now() terms; null until the first server response.
    let deadline: number | null = null;

    async function refreshDeadline() {
      if (refreshing) return;
      refreshing = true;
      try {
        const session = await apiClient.get<SessionResponse>(API_ROUTES.auth.session);
        if (!cancelled && session.expires_in > 0) {
          deadline = performance.now() + session.expires_in * 1000;
        }
      } catch {
        // A 401 is handled globally by api-client (redirect to login).
        // Network errors keep the previous deadline — never log out locally.
      } finally {
        refreshing = false;
      }
    }

    function check() {
      if (!getToken()) return;
      if (deadline === null) {
        void refreshDeadline();
        return;
      }

      const remainingMs = deadline - performance.now();

      if (remainingMs <= 0) {
        // Ask the server; expiry shows up as a 401 through api-client.
        void refreshDeadline();
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

    if (getToken()) {
      void refreshDeadline();
    }
    const id = setInterval(check, CHECK_INTERVAL_MS);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, []);
}
