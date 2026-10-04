import { describe, expect, it } from 'vitest';
import type { ConnectionMethod } from '@shared/api/network';
import {
  describeScheduleLockout,
  formatClock,
  nextOccurrence,
  parseClockTime,
  shouldWarnScheduleLockout,
} from '../wifi-schedule-lockout';

function at(hours: number, minutes: number): Date {
  const d = new Date(2026, 9, 4, hours, minutes, 0, 0);
  return d;
}

/**
 * The methods that reach the router over WiFi, taken from the API type rather
 * than hard-coded: the schedule's toggle helper (the generated
 * /usr/libexec/travo-wireless-toggle.sh) flips wireless.<device>.disabled for
 * EVERY wifi-device in /etc/config/wireless, so one tick takes down the uplink
 * STA radio and every access point alike. Every method that arrives over WiFi is
 * therefore locked out, and ethernet is the only survivor.
 *
 * `Record` over the wifi-* members of ConnectionMethod makes the list
 * exhaustive on purpose: a new WiFi connection method in the API type breaks
 * this file until it is classified, instead of quietly losing its warning.
 */
type WifiConnectionMethod = Extract<ConnectionMethod['method'], `wifi-${string}`>;
const WIFI_REACHABLE_METHODS: Record<WifiConnectionMethod, true> = {
  'wifi-client': true,
  'wifi-ap': true,
};

describe('wifi-schedule-lockout', () => {
  it('parses and rejects invalid clock times', () => {
    expect(parseClockTime('22:00')).toEqual({ hours: 22, minutes: 0 });
    expect(parseClockTime('7:05')).toEqual({ hours: 7, minutes: 5 });
    expect(parseClockTime('24:00')).toBeNull();
    expect(parseClockTime('22:60')).toBeNull();
    expect(parseClockTime('noon')).toBeNull();
  });

  it('finds the next occurrence of a time today or tomorrow', () => {
    expect(formatClock(nextOccurrence('22:00', at(21, 0))!)).toBe('22:00');
    expect(formatClock(nextOccurrence('22:00', at(22, 30))!)).toBe('22:00');
  });

  it('describes the next off and the following on time', () => {
    const lockout = describeScheduleLockout(
      { enabled: true, onTime: '08:00', offTime: '22:00' },
      at(21, 0),
    );
    expect(lockout).not.toBeNull();
    expect(formatClock(lockout!.offAt)).toBe('22:00');
    expect(formatClock(lockout!.onAt)).toBe('08:00');
    expect(lockout!.minutesUntilOff).toBe(60);
  });

  it('rolls the on time to the next day when it equals the off time', () => {
    const lockout = describeScheduleLockout(
      { enabled: true, onTime: '22:00', offTime: '22:00' },
      at(21, 0),
    );
    expect(lockout!.onAt.getDate()).toBeGreaterThan(lockout!.offAt.getDate());
  });

  it('returns nothing for a disabled schedule or an invalid time', () => {
    expect(
      describeScheduleLockout({ enabled: false, onTime: '08:00', offTime: '22:00' }),
    ).toBeNull();
    expect(describeScheduleLockout({ enabled: true, onTime: '08:00', offTime: 'nope' })).toBeNull();
  });

  it('warns a wifi-connected operator about an imminent off time', () => {
    const next = { enabled: true, onTime: '08:00', offTime: '22:00' };
    expect(
      shouldWarnScheduleLockout({
        next,
        current: null,
        connectionMethod: 'wifi-client',
        now: at(21, 0),
      }),
    ).toBe(true);
  });

  it('warns every operator who reaches the router over WiFi', () => {
    const next = { enabled: true, onTime: '08:00', offTime: '22:00' };
    for (const method of Object.keys(WIFI_REACHABLE_METHODS)) {
      const warned = shouldWarnScheduleLockout({
        next,
        current: null,
        connectionMethod: method,
        now: at(21, 0),
      });
      expect(
        warned,
        `connection method ${method} must be warned: the toggle helper disables every radio`,
      ).toBe(true);
    }
  });

  it('does not warn over a wired connection or when the method is unknown', () => {
    const next = { enabled: true, onTime: '08:00', offTime: '22:00' };
    for (const method of ['ethernet', 'unknown', undefined]) {
      const warned = shouldWarnScheduleLockout({
        next,
        current: null,
        connectionMethod: method,
        now: at(21, 0),
      });
      expect(warned).toBe(false);
    }
  });

  it('does not warn when the saved schedule is unchanged', () => {
    const schedule = { enabled: true, onTime: '08:00', offTime: '22:00' };
    expect(
      shouldWarnScheduleLockout({
        next: schedule,
        current: schedule,
        connectionMethod: 'wifi-client',
        now: at(21, 0),
      }),
    ).toBe(false);
  });

  it('stays quiet when the off time is beyond the warning window', () => {
    const next = { enabled: true, onTime: '08:00', offTime: '22:00' };
    expect(
      shouldWarnScheduleLockout({
        next,
        current: null,
        connectionMethod: 'wifi-client',
        warningWindowHours: 1,
        now: at(12, 0),
      }),
    ).toBe(false);
  });

  it('stays quiet when the save only turns the schedule off', () => {
    expect(
      shouldWarnScheduleLockout({
        next: { enabled: false, onTime: '08:00', offTime: '22:00' },
        current: { enabled: true, onTime: '08:00', offTime: '22:00' },
        connectionMethod: 'wifi-client',
        now: at(21, 0),
      }),
    ).toBe(false);
  });
});
