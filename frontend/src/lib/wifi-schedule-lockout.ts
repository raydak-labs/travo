/**
 * Lockout maths for the WiFi on/off schedule.
 *
 * The schedule turns the radios down on a wall-clock time, with no check of how
 * the operator is reaching the router. A WiFi-connected admin who saves an
 * enabled schedule with "Off at 22:00" at 21:00 is disconnected at 22:00 and
 * has no route back to this page until the On time — so the UI has to say so
 * before the save, not after.
 */

export interface ScheduleTimes {
  readonly enabled: boolean;
  /** HH:MM, 24h */
  readonly onTime: string;
  /** HH:MM, 24h */
  readonly offTime: string;
}

export interface ScheduleLockout {
  /** First instant the schedule switches WiFi off, at or after `now`. */
  readonly offAt: Date;
  /** First instant the schedule switches WiFi back on, at or after `offAt`. */
  readonly onAt: Date;
  /** Minutes from `now` until the first "off" instant. */
  readonly minutesUntilOff: number;
}

const MINUTE_MS = 60_000;

export function parseClockTime(value: string): { hours: number; minutes: number } | null {
  const match = /^(\d{1,2}):(\d{2})$/.exec(value.trim());
  if (!match) return null;
  const hours = Number(match[1]);
  const minutes = Number(match[2]);
  if (!Number.isInteger(hours) || !Number.isInteger(minutes)) return null;
  if (hours < 0 || hours > 23 || minutes < 0 || minutes > 59) return null;
  return { hours, minutes };
}

/** Next occurrence of `time` strictly after `from`, on `from`'s day or the next one. */
export function nextOccurrence(time: string, from: Date): Date | null {
  const parsed = parseClockTime(time);
  if (!parsed) return null;
  const candidate = new Date(from);
  candidate.setHours(parsed.hours, parsed.minutes, 0, 0);
  if (candidate.getTime() <= from.getTime()) {
    candidate.setDate(candidate.getDate() + 1);
  }
  return candidate;
}

export function formatClock(date: Date): string {
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  return `${hours}:${minutes}`;
}

/**
 * When the schedule next turns WiFi off and back on, or `null` when the times
 * are unparseable (the form schema rejects those before we get here).
 */
export function describeScheduleLockout(
  schedule: ScheduleTimes,
  now: Date = new Date(),
): ScheduleLockout | null {
  if (!schedule.enabled) return null;
  const offAt = nextOccurrence(schedule.offTime, now);
  if (!offAt) return null;
  const onAt = nextOccurrence(schedule.onTime, offAt);
  if (!onAt) return null;
  return {
    offAt,
    onAt,
    minutesUntilOff: Math.round((offAt.getTime() - now.getTime()) / MINUTE_MS),
  };
}

export interface ScheduleLockoutCheck {
  /** The schedule as it will be after this save. */
  readonly next: ScheduleTimes;
  /** The schedule as it is on the router right now, or null if never loaded. */
  readonly current: ScheduleTimes | null;
  /** ConnectionMethod.method of the session making the change. */
  readonly connectionMethod: string | undefined;
  /** Only warn for an off time this soon. Defaults to 24 hours. */
  readonly warningWindowHours?: number;
  readonly now?: Date;
}

/**
 * Whether the operator has to confirm before the schedule is committed: the
 * save would leave WiFi on and the session reaching the router over WiFi when
 * the schedule next switches it off.
 */
export function shouldWarnScheduleLockout({
  next,
  current,
  connectionMethod,
  warningWindowHours = 24,
  now = new Date(),
}: ScheduleLockoutCheck): boolean {
  if (connectionMethod !== 'wifi-client') return false;
  const lockout = describeScheduleLockout(next, now);
  if (!lockout) return false;
  if (lockout.minutesUntilOff > warningWindowHours * 60) return false;
  // Nothing actually changes for this operator, so a warning would be noise.
  if (current?.enabled && current.offTime === next.offTime) return false;
  return true;
}
