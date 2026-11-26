import type { StatsHistoryPoint, StatsHistoryUplinkBytes } from './api/gen';

/** One value per bucket end (`at`, unix seconds); null where the daemon recorded none. */
export type Sample = { at: number; value: number | null };
export type Uplink = StatsHistoryUplinkBytes['name'];

export type HistoryView = {
  /** rx+tx bytes per bucket, one series per uplink seen in the range, in API order. */
  traffic: { name: Uplink; total: number; samples: Sample[] }[];
  clients: Sample[];
  uptime: Sample[];
  /** Uptime went down between two points: the router restarted in between (ADR 0026). */
  reboots: number;
};

export type Summary = { min: number; max: number; last: number };

/** Splits the points into the series the history card draws. */
export function historyView(points: readonly StatsHistoryPoint[]): HistoryView {
  const names: Uplink[] = [];
  for (const point of points) {
    for (const uplink of point.uplinks) {
      if (!names.includes(uplink.name)) names.push(uplink.name);
    }
  }
  const traffic = names.map((name) => {
    const samples = points.map((point) => {
      const bytes = point.uplinks.find((uplink) => uplink.name === name);
      return { at: point.at, value: bytes ? bytes.rx_bytes + bytes.tx_bytes : null };
    });
    return { name, total: samples.reduce((sum, s) => sum + (s.value ?? 0), 0), samples };
  });
  let reboots = 0;
  points.reduce<StatsHistoryPoint | undefined>((previous, point) => {
    if (previous && point.uptime_seconds < previous.uptime_seconds) reboots++;
    return point;
  }, undefined);
  return {
    traffic,
    clients: points.map((point) => ({ at: point.at, value: point.clients })),
    uptime: points.map((point) => ({ at: point.at, value: point.uptime_seconds })),
    reboots,
  };
}

/** Min, max and last of the recorded values; null when there are none. */
export function summarize(samples: readonly Sample[]): Summary | null {
  const values = samples.flatMap((s) => (s.value === null ? [] : [s.value]));
  const last = values.at(-1);
  if (last === undefined) return null;
  return { min: Math.min(...values), max: Math.max(...values), last };
}

export type Frame = {
  width: number;
  height: number;
  /** Time window drawn left to right, unix seconds. */
  start: number;
  end: number;
  /** Bucket length; a step longer than 1.5 buckets is a gap, not a line. */
  intervalSeconds: number;
};

/**
 * SVG path data for a line through the samples: time on x, value on y (0 at the bottom).
 * A null value or a missing bucket breaks the line, so gaps stay visible. A lone sample
 * becomes a zero-length segment, which a round line cap draws as a dot.
 */
export function sparklinePath(samples: readonly Sample[], frame: Frame): string {
  const summary = summarize(samples);
  if (!summary) return '';
  const top = Math.max(summary.max, 1);
  const span = Math.max(frame.end - frame.start, 1);
  const x = (at: number) => round(((at - frame.start) / span) * frame.width);
  const y = (value: number) => round(frame.height - (Math.max(value, 0) / top) * frame.height);
  const parts: string[] = [];
  let previous: number | undefined;
  for (const sample of samples) {
    if (sample.value === null) {
      previous = undefined;
      continue;
    }
    const joined = previous !== undefined && sample.at - previous <= frame.intervalSeconds * 1.5;
    const point = `${x(sample.at)} ${y(sample.value)}`;
    parts.push(joined ? `L${point}` : `M${point}L${point}`);
    previous = sample.at;
  }
  return parts.join('');
}

function round(n: number): number {
  return Math.round(n * 10) / 10;
}
