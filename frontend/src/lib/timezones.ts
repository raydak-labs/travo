/** Known IANA timezone → POSIX timezone string mappings supported by OpenWRT. */
export const TIMEZONES = [
  { zonename: 'UTC', timezone: 'UTC0' },
  { zonename: 'Europe/London', timezone: 'GMT0BST,M3.5.0/1,M10.5.0' },
  { zonename: 'Europe/Berlin', timezone: 'CET-1CEST,M3.5.0,M10.5.0/3' },
  { zonename: 'Europe/Paris', timezone: 'CET-1CEST,M3.5.0,M10.5.0/3' },
  { zonename: 'Europe/Rome', timezone: 'CET-1CEST,M3.5.0,M10.5.0/3' },
  { zonename: 'Europe/Madrid', timezone: 'CET-1CEST,M3.5.0,M10.5.0/3' },
  { zonename: 'Europe/Moscow', timezone: 'MSK-3' },
  { zonename: 'America/New_York', timezone: 'EST5EDT,M3.2.0,M11.1.0' },
  { zonename: 'America/Chicago', timezone: 'CST6CDT,M3.2.0,M11.1.0' },
  { zonename: 'America/Denver', timezone: 'MST7MDT,M3.2.0,M11.1.0' },
  { zonename: 'America/Los_Angeles', timezone: 'PST8PDT,M3.2.0,M11.1.0' },
  { zonename: 'America/Sao_Paulo', timezone: '<-03>3' },
  { zonename: 'Asia/Tokyo', timezone: 'JST-9' },
  { zonename: 'Asia/Shanghai', timezone: 'CST-8' },
  { zonename: 'Asia/Kolkata', timezone: 'IST-5:30' },
  { zonename: 'Asia/Dubai', timezone: '<+04>-4' },
  { zonename: 'Asia/Singapore', timezone: '<+08>-8' },
  { zonename: 'Asia/Seoul', timezone: 'KST-9' },
  { zonename: 'Australia/Sydney', timezone: 'AEST-10AEDT,M10.1.0,M4.1.0/3' },
  { zonename: 'Pacific/Auckland', timezone: 'NZST-12NZDT,M9.5.0,M4.1.0/3' },
  { zonename: 'Africa/Cairo', timezone: 'EET-2EEST,M4.5.5/0,M10.5.4/24' },
  { zonename: 'Africa/Johannesburg', timezone: 'SAST-2' },
] as const;

export type TimezoneEntry = (typeof TIMEZONES)[number];

/** Look up the POSIX entry for a given IANA zonename. Returns undefined if not found. */
export function findTimezone(zonename: string): TimezoneEntry | undefined {
  return TIMEZONES.find((tz) => tz.zonename === zonename);
}
