export const TIMEZONE_DISMISS_KEY = 'timezone-mismatch-dismissed:v1';

export type TimezoneMismatch = {
  routerZone: string;
  browserZone: string;
};

/**
 * Reads the browser's IANA time zone. The resolver is injected for pure unit tests;
 * errors or empty values resolve to null.
 */
export function readBrowserTimezone(
  resolve: () => string | undefined = () => Intl.DateTimeFormat().resolvedOptions().timeZone,
): string | null {
  try {
    const raw = resolve();
    if (typeof raw !== 'string') return null;
    const zone = raw.trim();
    return zone === '' ? null : zone;
  } catch {
    return null;
  }
}

export function serializeDismissedPair(routerZone: string, browserZone: string): string {
  return JSON.stringify([routerZone, browserZone]);
}

/** sessionStorage can be absent or throw (private mode, blocked site data): treat as not dismissed. */
export function isTimezoneMismatchDismissed(
  storage: Pick<Storage, 'getItem'> | undefined,
  routerZone: string,
  browserZone: string,
): boolean {
  try {
    const raw = storage?.getItem(TIMEZONE_DISMISS_KEY);
    return raw === serializeDismissedPair(routerZone, browserZone);
  } catch {
    return false;
  }
}

export function dismissTimezoneMismatch(
  storage: Pick<Storage, 'setItem'> | undefined,
  routerZone: string,
  browserZone: string,
): void {
  try {
    storage?.setItem(TIMEZONE_DISMISS_KEY, serializeDismissedPair(routerZone, browserZone));
  } catch {
    // Failing to persist only drops the remembered dismissal for this session.
  }
}

/**
 * Reports a mismatch only when the browser time zone is non-empty, supported by the router's
 * zone list, differs from the router's current zonename, and has not been dismissed for this
 * (routerZone, browserZone) pair in session storage.
 */
export function detectTimezoneMismatch(input: {
  routerZone: string;
  zonenames: readonly string[];
  browserZone: string | null | undefined;
  storage?: Pick<Storage, 'getItem'>;
}): TimezoneMismatch | null {
  const browserZone = typeof input.browserZone === 'string' ? input.browserZone.trim() : '';
  if (browserZone === '') return null;
  if (!input.zonenames.includes(browserZone)) return null;
  if (browserZone === input.routerZone) return null;
  if (isTimezoneMismatchDismissed(input.storage, input.routerZone, browserZone)) return null;
  return { routerZone: input.routerZone, browserZone };
}
