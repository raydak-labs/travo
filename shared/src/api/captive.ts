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

export interface CaptiveAutoAcceptResult {
  readonly ok: boolean;
  readonly message?: string;
  readonly detected: boolean;
  readonly can_reach_internet: boolean;
  readonly portal_url?: string;
  readonly attempts?: number;
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
