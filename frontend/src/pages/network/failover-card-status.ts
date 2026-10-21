/**
 * The status of a failover candidate and of the mwan3 service, as the label
 * and the pill tone the card shows.
 */
import type { StatusPillProps } from '@/components/ui/status-pill';
import type { FailoverCandidate, FailoverConfig, FailoverTrackingState } from '@shared/index';

/**
 * The tracking state as something a user can act on. `not_available` vs
 * `not_installed` is a distinction only the mwan3 author cares about, and
 * neither tells a user what to do.
 */
export const TRACKING_LABEL: Record<FailoverTrackingState, string> = {
  online: 'Online',
  offline: 'Offline',
  disabled: 'Skipped',
  not_installed: 'Service missing',
  not_available: 'No device',
  unknown: 'Unknown',
};

export function trackingLabel(candidate: FailoverCandidate): string {
  if (!candidate.enabled) return 'Skipped';
  return TRACKING_LABEL[candidate.tracking_state] ?? candidate.tracking_state;
}

/**
 * A tracking state is a status, not a caption: it gets the shared pill so
 * "Online" in the order list is the same green as "Online" anywhere else. A
 * disabled candidate is the operator's own choice, so it reads as neutral
 * rather than as a problem.
 */
export function trackingTone(candidate: FailoverCandidate): StatusPillProps['tone'] {
  if (!candidate.enabled) return 'neutral';
  switch (candidate.tracking_state) {
    case 'online':
      return 'ok';
    case 'offline':
      return 'danger';
    case 'not_installed':
    case 'not_available':
      return 'warn';
    case 'disabled':
      return 'neutral';
    default:
      return 'stale';
  }
}

/** The mwan3 service state as one pill: missing service, on, or off. */
export function operatorStateTone(config: FailoverConfig): StatusPillProps['tone'] {
  if (!config.service_installed) return 'warn';
  return config.enabled ? 'ok' : 'neutral';
}
