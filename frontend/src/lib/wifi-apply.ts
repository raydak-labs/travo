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

/**
 * Substring the backend sends when ConfirmApply could not READ netifd's answer
 * (services.ErrWirelessUnverifiable) — the `ubus call` itself failed, or it
 * answered in a shape this build does not understand. Deliberately not a prefix
 * of WIRELESS_NOT_VERIFIED: the two conditions need different answers, and the
 * shared prefix used to make an unreadable answer throw
 * WifiApplyRolledBackError — telling the operator the router had already rolled
 * back while rpcd's rollback window was still open and nothing had happened yet.
 */
const WIRELESS_UNVERIFIABLE = 'wireless apply could not be verified';

export function isWirelessNotVerifiedError(error: unknown): boolean {
  return error instanceof Error && error.message.toLowerCase().includes(WIRELESS_NOT_VERIFIED);
}

export function isWirelessStatusUnreadableError(error: unknown): boolean {
  return error instanceof Error && error.message.toLowerCase().includes(WIRELESS_UNVERIFIABLE);
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

/**
 * Travo could not read whether the new wireless settings came up — the ubus
 * call to netifd failed, or the router answered in a shape this build does not
 * understand. Nothing is known about the change either way: the rollback window
 * is STILL OPEN, and rpcd rolls it back when the window expires. The message must
 * not claim a rollback that has not happened, and must not send the operator
 * looking at their own configuration, which may well be fine.
 */
export class WifiApplyUnverifiedError extends Error {
  constructor() {
    super(
      'Travo could not verify that the new wireless settings came up, so the change was ' +
        'not confirmed. If the router does not come up on them it will go back to the ' +
        'previous settings on its own.',
    );
    this.name = 'WifiApplyUnverifiedError';
  }
}

/**
 * Probe budget assumed when apply.probe_budget_seconds is missing (a backend
 * that predates the field) or is not a usable positive number. Same order of
 * magnitude as the current device value, so an old backend costs one or two
 * retries rather than a probe answered after the rollback.
 *
 * Sized like the current device value: the backend waits ~10 s for netifd to
 * link the new interfaces up (measured on the device: ACS + link-up) and adds
 * the ubus round-trips its per-interface fallback makes, which publishes 12 s.
 * The reservation below must stay inside half the 30 s rollback window; the
 * margin on top of it is what leaves real headroom for the last probe.
 */
export const DEFAULT_PROBE_BUDGET_SECONDS = 12;

/**
 * Room left for request transit, scheduling jitter and the rollback itself. No
 * field in the apply envelope describes it: the probe budget bounds how long the
 * device holds the call, not how long the request takes to reach it and come
 * back. 2 s, and the same number the backend's budget gate uses
 * (wirelessProbeSafetyMarginSeconds in backend/internal/services/wifi_service.go);
 * they used to disagree (500 ms here, 1 s there, "half a second" in the
 * comment). TestClientProbeSafetyMarginMatchesTheGoGate in
 * backend/internal/services/wifi_service_test.go fails if they ever drift again.
 */
export const PROBE_SAFETY_MARGIN_MS = 2000;

/**
 * The last instant at which a confirm probe may still be issued.
 *
 * rpcd rolls the change back `rollbackTimeoutSeconds` after the apply, and one
 * confirm call blocks on the device for up to `probeBudgetSeconds` while
 * netifd brings the new interfaces up. A probe issued after this deadline is
 * answered only after rpcd has already rolled back: the confirm lands on a dead
 * session and the operator gets a confusing failure instead of a clean
 * rollback, so the budget is subtracted rather than ignored. The last probe
 * therefore still finishes PROBE_SAFETY_MARGIN_MS before the rollback fires,
 * which is the headroom that margin is for.
 *
 * Neither degenerate input may break the loop: a missing or nonsensical budget
 * must not yield a NaN deadline (every comparison against NaN is false, which
 * silently skips all probing) nor a deadline in the past (same). So the budget
 * falls back to DEFAULT_PROBE_BUDGET_SECONDS, is never allowed to eat more than
 * half the rollback window, and never starts the deadline before `now`.
 */
export function confirmDeadlineMs(
  rollbackTimeoutSeconds: number,
  probeBudgetSeconds: number | undefined,
  now: number,
): number {
  const rollbackWindowMs = Math.max(0, rollbackTimeoutSeconds) * 1000;
  const budget =
    typeof probeBudgetSeconds === 'number' &&
    Number.isFinite(probeBudgetSeconds) &&
    probeBudgetSeconds > 0
      ? probeBudgetSeconds
      : DEFAULT_PROBE_BUDGET_SECONDS;
  const reserved = Math.min(budget * 1000 + PROBE_SAFETY_MARGIN_MS, rollbackWindowMs / 2);
  return now + rollbackWindowMs - reserved;
}

export async function confirmWifiApply(
  token: string,
  rollbackTimeoutSeconds = 30,
  intervalMs = 1500,
  probeBudgetSeconds: number = DEFAULT_PROBE_BUDGET_SECONDS,
): Promise<void> {
  const deadline = confirmDeadlineMs(rollbackTimeoutSeconds, probeBudgetSeconds, Date.now());
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
      // Also final, and for a different reason: the device could not read its
      // own answer (the ubus call failed, or the payload is unreadable), so
      // nothing is known about the change. Retrying only burns the window, and
      // it must not be reported as a rollback that has not happened.
      if (isWirelessStatusUnreadableError(error)) {
        throw new WifiApplyUnverifiedError();
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
    await confirmWifiApply(
      apply.token,
      apply.rollback_timeout_seconds ?? 30,
      1500,
      apply.probe_budget_seconds ?? DEFAULT_PROBE_BUDGET_SECONDS,
    );
  }
  return response;
}
