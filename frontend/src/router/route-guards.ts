import { redirect } from '@tanstack/react-router';
import { getToken, apiClient } from '@/lib/api-client';
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
    const data = await apiClient.get<{ complete: boolean }>(API_ROUTES.system.setupComplete);
    if (!data.complete) {
      throw redirect({ to: '/setup' });
    }
  } catch (e: unknown) {
    if (e !== null && typeof e === 'object' && 'to' in e) throw e;
  }
}
