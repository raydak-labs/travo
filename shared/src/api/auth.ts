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

/** Change password request payload */
export interface ChangePasswordRequest {
  readonly current_password: string;
  readonly new_password: string;
}

/** Active session */
export interface Session {
  readonly token: string;
  readonly expires_at: string;
  readonly created_at: string;
}
