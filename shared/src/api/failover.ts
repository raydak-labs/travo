import type { FailoverCandidate } from '../generated/schemas';

// Generated from the backend's OpenAPI document (make generate).
export type {
  FailoverCandidate,
  FailoverConfig,
  FailoverEvent,
  FailoverHealthConfig,
} from '../generated/schemas';

export type FailoverTrackingState = FailoverCandidate['tracking_state'];
