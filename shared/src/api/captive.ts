import type { CaptiveAutoAcceptResult, CaptivePortalStatus } from '../generated/schemas';

// Generated from the backend's OpenAPI document (make generate).
export type { CaptiveAutoAcceptResult, CaptivePortalStatus } from '../generated/schemas';

/** Type guard for CaptivePortalStatus */
export function isCaptivePortalStatus(value: unknown): value is CaptivePortalStatus {
  if (typeof value !== 'object' || value === null) return false;
  const v = value as Record<string, unknown>;
  return (
    typeof v.detected === 'boolean' &&
    typeof v.can_reach_internet === 'boolean' &&
    typeof v.sta_connected === 'boolean'
  );
}

export function isCaptiveAutoAcceptResult(value: unknown): value is CaptiveAutoAcceptResult {
  if (typeof value !== 'object' || value === null) return false;
  const v = value as Record<string, unknown>;
  return (
    typeof v.ok === 'boolean' &&
    typeof v.detected === 'boolean' &&
    typeof v.can_reach_internet === 'boolean'
  );
}
