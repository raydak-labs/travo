// Generated from the backend's OpenAPI document (make generate).
export type {
  FailoverCandidate,
  FailoverConfig,
  FailoverEvent,
  FailoverHealthConfig,
} from '../generated/schemas';

export type FailoverCandidateKind = 'ethernet' | 'wifi' | 'usb';

export type FailoverTrackingState =
  'online' | 'offline' | 'disabled' | 'not_installed' | 'not_available' | 'unknown';
