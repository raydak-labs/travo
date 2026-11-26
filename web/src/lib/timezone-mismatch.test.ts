import { describe, expect, it } from 'vitest';
import {
  detectTimezoneMismatch,
  dismissTimezoneMismatch,
  isTimezoneMismatchDismissed,
  readBrowserTimezone,
  TIMEZONE_DISMISS_KEY,
} from './timezone-mismatch';

function memoryStorage(initial: Record<string, string> = {}) {
  const map = new Map(Object.entries(initial));
  return {
    getItem: (key: string) => map.get(key) ?? null,
    setItem: (key: string, value: string) => {
      map.set(key, value);
    },
  };
}

const ZONES = ['UTC', 'America/New_York', 'Asia/Tokyo', 'Europe/Berlin'] as const;

describe('readBrowserTimezone', () => {
  it('returns the trimmed IANA time zone when available', () => {
    expect(readBrowserTimezone(() => ' Europe/Berlin ')).toBe('Europe/Berlin');
  });

  it('returns null when empty, undefined, or when Intl throws', () => {
    expect(readBrowserTimezone(() => '')).toBeNull();
    expect(readBrowserTimezone(() => '   ')).toBeNull();
    expect(readBrowserTimezone(() => undefined)).toBeNull();
    expect(
      readBrowserTimezone(() => {
        throw new Error('Intl unavailable');
      }),
    ).toBeNull();
  });
});

describe('detectTimezoneMismatch', () => {
  it('returns null when router and browser time zones match', () => {
    expect(
      detectTimezoneMismatch({
        routerZone: 'Europe/Berlin',
        zonenames: ZONES,
        browserZone: 'Europe/Berlin',
      }),
    ).toBeNull();
  });

  it('reports a mismatch when the browser zone is supported and differs from the router', () => {
    expect(
      detectTimezoneMismatch({
        routerZone: 'UTC',
        zonenames: ZONES,
        browserZone: 'Europe/Berlin',
      }),
    ).toEqual({ routerZone: 'UTC', browserZone: 'Europe/Berlin' });
  });

  it('returns null when the browser zone is empty, null, undefined, or unsupported', () => {
    for (const browserZone of [null, undefined, '', '   ', 'Mars/Olympus', 'US/Eastern']) {
      expect(
        detectTimezoneMismatch({
          routerZone: 'UTC',
          zonenames: ZONES,
          browserZone,
        }),
      ).toBeNull();
    }
  });

  it('returns null when the router zonenames list is empty', () => {
    expect(
      detectTimezoneMismatch({
        routerZone: 'UTC',
        zonenames: [],
        browserZone: 'Europe/Berlin',
      }),
    ).toBeNull();
  });

  it('suppresses a dismissed (routerZone, browserZone) pair in session storage', () => {
    const storage = memoryStorage();
    dismissTimezoneMismatch(storage, 'UTC', 'Europe/Berlin');

    expect(isTimezoneMismatchDismissed(storage, 'UTC', 'Europe/Berlin')).toBe(true);
    expect(
      detectTimezoneMismatch({
        routerZone: 'UTC',
        zonenames: ZONES,
        browserZone: 'Europe/Berlin',
        storage,
      }),
    ).toBeNull();
  });

  it('shows again when either the browser zone or router zone changes', () => {
    const storage = memoryStorage();
    dismissTimezoneMismatch(storage, 'UTC', 'Europe/Berlin');

    expect(
      detectTimezoneMismatch({
        routerZone: 'UTC',
        zonenames: ZONES,
        browserZone: 'Asia/Tokyo',
        storage,
      }),
    ).toEqual({ routerZone: 'UTC', browserZone: 'Asia/Tokyo' });

    expect(
      detectTimezoneMismatch({
        routerZone: 'America/New_York',
        zonenames: ZONES,
        browserZone: 'Europe/Berlin',
        storage,
      }),
    ).toEqual({ routerZone: 'America/New_York', browserZone: 'Europe/Berlin' });
  });

  it('tolerates throwing or malformed storage without failing', () => {
    const broken = {
      getItem: () => {
        throw new Error('blocked');
      },
      setItem: () => {
        throw new Error('blocked');
      },
    };

    expect(() => dismissTimezoneMismatch(broken, 'UTC', 'Europe/Berlin')).not.toThrow();
    expect(isTimezoneMismatchDismissed(broken, 'UTC', 'Europe/Berlin')).toBe(false);
    expect(
      detectTimezoneMismatch({
        routerZone: 'UTC',
        zonenames: ZONES,
        browserZone: 'Europe/Berlin',
        storage: broken,
      }),
    ).toEqual({ routerZone: 'UTC', browserZone: 'Europe/Berlin' });

    const malformed = memoryStorage({ [TIMEZONE_DISMISS_KEY]: 'not-json' });
    expect(
      detectTimezoneMismatch({
        routerZone: 'UTC',
        zonenames: ZONES,
        browserZone: 'Europe/Berlin',
        storage: malformed,
      }),
    ).toEqual({ routerZone: 'UTC', browserZone: 'Europe/Berlin' });
  });
});
