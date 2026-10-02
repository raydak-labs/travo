/** Login request payload */
export interface LoginRequest {
  readonly password: string;
}

/** Login response payload */
export interface LoginResponse {
  readonly token: string;
  readonly expires_at: string;
  /** Relative session lifetime in seconds — safe across clock/timezone skew. */
  readonly expires_in: number;
}

/** Session status response payload (GET /auth/session) */
export interface SessionResponse {
  readonly valid: boolean;
  /** Remaining session lifetime in seconds, relative to the server's clock. */
  readonly expires_in: number;
}

/**
 * Minimum accepted admin password length.
 *
 * Mirrors the backend policy (backend/internal/auth.MinPasswordLength). The
 * two must stay in step: a frontend that accepts a shorter password than the
 * server shows the user a validation error only after submitting.
 */
export const MIN_PASSWORD_LENGTH = 8;

/** Change password request payload */
export interface ChangePasswordRequest {
  readonly current_password: string;
  readonly new_password: string;
}

/**
 * Change password response.
 *
 * The server revokes every live session on a password change (so a token
 * stolen beforehand stays dead), blocklists the caller's superseded token and
 * issues a fresh one. Clients MUST store {@link token} from this response —
 * keeping the old token logs the user out on the next request.
 */
export interface ChangePasswordResponse {
  readonly status: string;
  readonly token: string;
  readonly expires_at: string;
  readonly expires_in: number;
  /** Live sessions invalidated by the change, including the caller's own. */
  readonly revoked_sessions: number;
}

/** Active session */
export interface Session {
  readonly token: string;
  readonly expires_at: string;
  readonly created_at: string;
}
