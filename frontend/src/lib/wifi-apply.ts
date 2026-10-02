import { API_ROUTES } from '@shared/index';
import type { WifiMutationResponse } from '@shared/index';
import { apiClient, ApiError } from '@/lib/api-client';

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * Whether a failed apply-confirm can possibly succeed on a later attempt.
 *
 * Retrying only makes sense while the router is unreachable or busy. A 4xx is
 * the device's final answer: 400/401/403 in particular were previously
 * swallowed and re-POSTed ~20 times, which cleared the session token ~20 times
 * and fired ~20 `location.assign('/login')` redirects. 408 and 429 stay
 * retryable because they are transient by definition.
 */
export function isTerminalApplyStatus(status: number): boolean {
  if (status < 400 || status >= 500) return false;
  return status !== 408 && status !== 429;
}

export async function confirmWifiApply(
  token: string,
  rollbackTimeoutSeconds = 30,
  intervalMs = 1500,
): Promise<void> {
  const deadline = Date.now() + rollbackTimeoutSeconds * 1000;
  let lastError: unknown;

  while (Date.now() <= deadline) {
    try {
      await apiClient.post<{ status: string }>(API_ROUTES.wifi.applyConfirm, { token });
      return;
    } catch (error) {
      lastError = error;
      if (error instanceof ApiError && isTerminalApplyStatus(error.status)) {
        // Surface the device's answer immediately instead of waiting for the
        // rollback window to expire.
        throw error;
      }
      if (Date.now() + intervalMs > deadline) {
        break;
      }
      await sleep(intervalMs);
    }
  }

  const suffix = lastError instanceof Error && lastError.message ? `: ${lastError.message}` : '';
  throw new Error(`Wireless settings could not be confirmed before rollback timeout${suffix}`);
}

export async function finalizeWifiMutation<T extends WifiMutationResponse>(
  promise: Promise<T>,
): Promise<T> {
  const response = await promise;
  const apply = response.apply;
  if (apply?.pending && apply.token) {
    await confirmWifiApply(apply.token, apply.rollback_timeout_seconds ?? 30);
  }
  return response;
}
