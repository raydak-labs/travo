/** Captive portal detection status */
export interface CaptivePortalStatus {
  readonly detected: boolean;
  readonly portal_url?: string;
  readonly can_reach_internet: boolean;
}

/** Type guard for CaptivePortalStatus */
export function isCaptivePortalStatus(value: unknown): value is CaptivePortalStatus {
  if (typeof value !== 'object' || value === null) return false;
  const v = value as Record<string, unknown>;
  return typeof v.detected === 'boolean' && typeof v.can_reach_internet === 'boolean';
}
