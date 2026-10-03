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

/**
 * Substring the backend sends when ConfirmApply found that the access points the
 * new config enables never came up (services.ErrWirelessNotUp). The change was
 * NOT applied — rpcd's rollback window was left to expire, so the router is
 * already back on the previous configuration.
 */
const WIRELESS_NOT_VERIFIED = 'wireless apply not verified';

export function isWirelessNotVerifiedError(error: unknown): boolean {
  return error instanceof Error && error.message.toLowerCase().includes(WIRELESS_NOT_VERIFIED);
}

/**
 * The wireless change was rejected by the device's own verification and the
 * router rolled back to the previous configuration. Distinct from a plain
 * failure so the UI can say "rolled back" instead of "something went wrong".
 */
export class WifiApplyRolledBackError extends Error {
  constructor() {
    super(
      'The router could not bring the new wireless settings up, so it rolled back ' +
        'to the previous settings.',
    );
    this.name = 'WifiApplyRolledBackError';
  }
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
      // A final answer even though it arrives as a 5xx: the device already
      // decided the change is not applied, so retrying only burns the 30 s and
      // hides the fact that the router is back on the old settings.
      if (isWirelessNotVerifiedError(error)) {
        throw new WifiApplyRolledBackError();
      }
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
