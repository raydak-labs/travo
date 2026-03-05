/**
 * Shared types and utilities for openwrt-travel-gui.
 *
 * This package provides the single source of truth for API contracts
 * between the frontend and backend.
 */

/** Current API version */
export const API_VERSION = 'v1';

/** Standard health check response */
export interface HealthResponse {
  status: 'ok';
}

/** Type guard for HealthResponse */
export function isHealthResponse(value: unknown): value is HealthResponse {
  return (
    typeof value === 'object' &&
    value !== null &&
    'status' in value &&
    (value as Record<string, unknown>).status === 'ok'
  );
}

export * from './api';
