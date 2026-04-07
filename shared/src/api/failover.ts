export type FailoverCandidateKind = 'ethernet' | 'wifi' | 'usb';

export type FailoverTrackingState =
  | 'online'
  | 'offline'
  | 'disabled'
  | 'not_installed'
  | 'not_available'
  | 'unknown';

export interface FailoverHealthConfig {
  readonly track_ips: readonly string[];
  readonly reliability: number;
  readonly count: number;
  readonly timeout: number;
  readonly interval: number;
  readonly failure_interval: number;
  readonly recovery_interval: number;
  readonly down: number;
  readonly up: number;
}

export interface FailoverCandidate {
  readonly id: string;
  readonly label: string;
  readonly interface_name: string;
  readonly kind: FailoverCandidateKind;
  readonly available: boolean;
  readonly enabled: boolean;
  readonly priority: number;
  readonly tracking_state: FailoverTrackingState;
  readonly is_up: boolean;
}

export interface FailoverEvent {
  readonly from_interface: string;
  readonly to_interface: string;
  readonly timestamp: number;
  readonly reason: string;
}

export interface FailoverConfig {
  readonly available: boolean;
  readonly service_installed: boolean;
  readonly enabled: boolean;
  readonly active_interface: string;
  readonly candidates: readonly FailoverCandidate[];
  readonly health: FailoverHealthConfig;
  readonly last_failover_event?: FailoverEvent;
}
