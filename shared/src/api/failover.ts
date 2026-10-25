import type { FailoverCandidate } from '../generated/model';

// Generated from the backend's OpenAPI document (make generate).
export type {
  FailoverCandidate,
  FailoverConfig,
  FailoverEvent,
  FailoverHealthConfig,
} from '../generated/model';

export type FailoverTrackingState = FailoverCandidate['tracking_state'];
