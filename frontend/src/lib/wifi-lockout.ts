/**
 * The wireless lockout refusal (ADR 0002 §5).
 *
 * The backend answers 409 with this code when a wireless change would leave the
 * operator who asked for it with no access point to reconnect through. The
 * message says why; this code is what the UI keys off, so rewording the message
 * cannot silently turn the acknowledgement dialog off.
 */
export const WIFI_LOCKOUT_ERROR_CODE = 'wifi_lockout_risk';

/** A mutating wireless request body that can acknowledge the lockout. */
export type Acknowledgeable = { acknowledge_lockout?: boolean };

/**
 * Whether this failure is the lockout refusal.
 *
 * Only the code counts. Matching on the message would break the moment the
 * message is reworded to mention the remedy more precisely.
 */
export function isWifiLockoutError(error: unknown): boolean {
  return (
    typeof error === 'object' &&
    error !== null &&
    'code' in error &&
    (error as { code?: unknown }).code === WIFI_LOCKOUT_ERROR_CODE
  );
}
