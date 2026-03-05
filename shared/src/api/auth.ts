/** Login request payload */
export interface LoginRequest {
  readonly password: string;
}

/** Login response payload */
export interface LoginResponse {
  readonly token: string;
  readonly expires_at: string;
}

/** Active session */
export interface Session {
  readonly token: string;
  readonly expires_at: string;
  readonly created_at: string;
}
