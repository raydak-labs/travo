import { redirect } from '@tanstack/react-router';
import { getToken } from '@/lib/api-client';
import { currentRelativeLocation } from '@/lib/auth-redirect';
import { getSetupComplete } from '@/lib/setup-status';

/**
 * Sends the user to `/login` remembering where they were headed, so a deep
 * link survives an expired session instead of silently becoming the dashboard.
 */
function redirectToLogin(): never {
  throw redirect({ to: '/login', search: { redirect: currentRelativeLocation() } });
}

export function requireAuth() {
  if (!getToken()) {
    redirectToLogin();
  }
}

/**
 * Check setup status and redirect to /setup if not complete.
 *
 * The answer is cached (see `getSetupComplete`) so navigation does not refetch
 * it. An unavailable router is *not* treated as "setup complete": the error is
 * re-thrown so the route surfaces an error instead of quietly unlocking the
 * app.
 */
export async function requireSetupComplete() {
  requireAuth();

  // Any failure (5xx, transport) propagates: rendering the protected route as
  // if setup were complete would expose the app behind a half-configured
  // router, which is exactly what the old `catch {}` did.
  const complete = await getSetupComplete();

  if (!complete) {
    throw redirect({ to: '/setup', search: { redirect: currentRelativeLocation() } });
  }
}
