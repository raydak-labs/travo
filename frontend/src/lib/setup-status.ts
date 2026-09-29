import { apiClient, ApiError } from '@/lib/api-client';
import { API_ROUTES } from '@shared/index';
import type { SetupStatus } from '@shared/index';

/**
 * Cached answer for `GET /system/setup-complete`.
 *
 * The guard runs on every navigation, so re-requesting it each time both
 * slowed navigation down and turned a transient router hiccup into a wrong
 * render. The result is memoised for a short window; `resetSetupStatusCache`
 * must be called whenever setup state can change.
 */
const TTL_MS = 30_000;

let cached: { value: boolean; at: number } | null = null;
let inFlight: Promise<boolean> | null = null;

/** Thrown when setup status could not be determined; callers must not guess. */
export class SetupStatusUnavailableError extends Error {
  constructor(readonly cause?: unknown) {
    super('Setup status is temporarily unavailable');
    this.name = 'SetupStatusUnavailableError';
  }
}

export function resetSetupStatusCache(): void {
  cached = null;
  inFlight = null;
}

/**
 * Returns whether first-run setup is complete.
 *
 * Rejects with `SetupStatusUnavailableError` when the router cannot answer —
 * treating a 5xx or a transport failure as "setup complete" would expose the
 * whole app behind a half-configured router.
 */
export async function getSetupComplete(force = false): Promise<boolean> {
  if (!force && cached && Date.now() - cached.at < TTL_MS) {
    return cached.value;
  }
  if (!force && inFlight) return inFlight;

  const request = (async () => {
    try {
      const data = await apiClient.get<SetupStatus>(API_ROUTES.system.setupComplete);
      cached = { value: data.complete, at: Date.now() };
      return data.complete;
    } catch (error) {
      // A 401 is already handled globally (session cleared, redirect to login);
      // it must not be cached as "complete".
      throw new SetupStatusUnavailableError(error);
    } finally {
      inFlight = null;
    }
  })();

  inFlight = request;
  return request;
}

/** Human-readable reason, for logs and error surfaces. */
export function setupStatusErrorDetail(error: unknown): string {
  if (error instanceof SetupStatusUnavailableError && error.cause instanceof ApiError) {
    return `setup check failed with status ${error.cause.status}`;
  }
  if (error instanceof Error) return error.message;
  return 'unknown error';
}
