// Generated from the backend's OpenAPI document (make generate).
export type {
  ChangePasswordRequest,
  ChangePasswordResponse,
  LoginRequest,
  LoginResponse,
  SessionResponse,
} from '../generated/model';

/**
 * Minimum accepted admin password length.
 *
 * Mirrors the backend policy (backend/internal/auth.MinPasswordLength). The
 * two must stay in step: a frontend that accepts a shorter password than the
 * server shows the user a validation error only after submitting.
 */
export const MIN_PASSWORD_LENGTH = 8;
