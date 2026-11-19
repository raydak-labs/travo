import type { DataUsageEvent, StatsHistoryDay, StatsHistoryUplinkBytes } from './api/gen';
import type { Sample } from './stats-history';

const DAY = 86400;
const GIB = 1024 ** 3;

export type UplinkTotal = {
  name: StatsHistoryUplinkBytes['name'];
  rx: number;
  tx: number;
  total: number;
};

/** Rows for a period table, both directions summed. */
export function uplinkTotals(uplinks: readonly StatsHistoryUplinkBytes[]): UplinkTotal[] {
  return uplinks.map((u) => ({
    name: u.name,
    rx: u.rx_bytes,
    tx: u.tx_bytes,
    total: u.rx_bytes + u.tx_bytes,
  }));
}

/** Unix seconds of a day key (YYYY-MM-DD) read as UTC midnight: a calendar position for the chart. */
export function dayStart(date: string): number {
  return Date.parse(`${date}T00:00:00Z`) / 1000;
}

/**
 * One sample per day for an uplink, at the day's start. A day the store has but this
 * uplink is absent from is 0; a day the store lacks (nothing recorded) is a gap.
 */
export function dailySamples(days: readonly StatsHistoryDay[], name: string): Sample[] {
  const first = days[0];
  const last = days.at(-1);
  if (!first || !last) return [];
  const byDate = new Map(days.map((d) => [d.date, d]));
  const out: Sample[] = [];
  for (let at = dayStart(first.date); at <= dayStart(last.date); at += DAY) {
    const date = new Date(at * 1000).toISOString().slice(0, 10);
    const day = byDate.get(date);
    const bytes = day?.uplinks.find((u) => u.name === name);
    out.push({ at, value: day ? (bytes ? bytes.rx_bytes + bytes.tx_bytes : 0) : null });
  }
  return out;
}

/** The sparkline window over the retained days; undefined without days. */
export function dayFrame(days: readonly StatsHistoryDay[]) {
  const first = days[0];
  const last = days.at(-1);
  if (!first || !last) return undefined;
  return { start: dayStart(first.date), end: dayStart(last.date), intervalSeconds: DAY };
}

/** Uplinks with traffic on any retained day, in API order. */
export function dayUplinks(days: readonly StatsHistoryDay[]): StatsHistoryUplinkBytes['name'][] {
  const names: StatsHistoryUplinkBytes['name'][] = [];
  for (const d of days) {
    for (const u of d.uplinks) if (!names.includes(u.name)) names.push(u.name);
  }
  return names;
}

/** Used share of the limit in whole percent, capped for display at 999. */
export function usedPercent(used: number, limit: number): number {
  if (limit <= 0) return 0;
  return Math.min(999, Math.floor((used * 100) / limit));
}

/** The limit field works in GiB; the API in bytes. */
export function gibToBytes(gib: number): number {
  return Math.round(gib * GIB);
}

export function bytesToGib(bytes: number): number {
  return Math.round((bytes / GIB) * 100) / 100;
}

/** Newest first, at most `limit` entries. */
export function recentEvents(events: readonly DataUsageEvent[], limit = 20): DataUsageEvent[] {
  return [...events].reverse().slice(0, limit);
}
