export const TIMEZONE_DISMISS_KEY = 'timezone-mismatch-dismissed:v1';

export type TimezoneMismatch = {
  routerZone: string;
  browserZone: string;
};

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
    // Ignore storage errors (private mode / blocked site data).
  }
}

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
