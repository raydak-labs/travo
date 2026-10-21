import type { FailoverCandidate, FailoverConfig } from '@shared/index';

/**
 * The editable draft of the failover configuration: a deep copy the operator
 * can reorder and edit without touching the stored config, and the summary
 * the card renders for the tracked addresses.
 */
export function cloneConfig(config: FailoverConfig): FailoverConfig {
  return {
    ...config,
    candidates: config.candidates.map((candidate) => ({ ...candidate })),
    health: {
      ...config.health,
      track_ips: [...config.health.track_ips],
    },
    last_failover_event: config.last_failover_event ? { ...config.last_failover_event } : undefined,
  };
}

export function moveCandidate(
  candidates: readonly FailoverCandidate[],
  index: number,
  direction: -1 | 1,
) {
  const next = candidates.map((candidate) => ({ ...candidate }));
  const target = index + direction;
  if (target < 0 || target >= next.length) {
    return next;
  }
  [next[index], next[target]] = [next[target], next[index]];
  return next.map((candidate, candidateIndex) => ({
    ...candidate,
    priority: candidateIndex + 1,
  }));
}

export function summarizeTrackIPs(trackIPs: readonly string[]) {
  if (trackIPs.length === 0) return '—';
  if (trackIPs.length === 1) return trackIPs[0];
  return `${trackIPs.length} addresses`;
}

export const HEALTH_FIELDS = [
  { field: 'interval', label: 'Interval (s)', min: 1 },
  { field: 'down', label: 'Failures before down', min: 1 },
  { field: 'up', label: 'Successes before recovery', min: 1 },
] as const;
