import { redirect } from '@tanstack/react-router';
import { getToken } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';

export function requireAuth() {
  if (!getToken()) {
    throw redirect({ to: '/login' });
  }
}

/** Check setup status and redirect to /setup if not complete */
export async function requireSetupComplete() {
  requireAuth();
  try {
    const res = await fetch(API_ROUTES.system.setupComplete, {
      headers: { Authorization: `Bearer ${getToken()}` },
    });
    if (res.ok) {
      const data = (await res.json()) as { complete: boolean };
      if (!data.complete) {
        throw redirect({ to: '/setup' });
      }
    }
  } catch (e: unknown) {
    if (e !== null && typeof e === 'object' && 'to' in e) throw e;
  }
}
