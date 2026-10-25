import type { BandSwitchConfig, BandSwitchStatus, WifiScanResult } from '../generated/schemas';

// Generated from the backend's OpenAPI document (make generate).
export type {
  APConfig,
  APConfigUpdate,
  BandSwitchConfig,
  BandSwitchStatus,
  GuestWifiConfig,
  MACConfig,
  MACPolicies,
  MACPolicy,
  RadioInfo,
  RandomizeMACResponse,
  RepeaterOptions,
  SavedNetwork,
  SetMACRequest,
  WiFiSchedule,
  WifiConfig,
  WifiConnection,
  WifiHealth,
  WifiMutationResponse,
  WifiScanResult,
} from '../generated/schemas';

/** WiFi operation mode */
export type WifiMode = 'client' | 'ap' | 'repeater';

/** WiFi encryption type */
export type WifiEncryption = 'none' | 'wep' | 'wpa' | 'wpa2' | 'wpa3' | 'wpa2/wpa3';

/** WiFi frequency band */
export type WifiBand = '2.4ghz' | '5ghz' | '6ghz';

/** Group of scan results with same SSID and encryption (dual-band = one group) */
export interface GroupedScanNetwork {
  readonly ssid: string;
  /** Encryption from scan (e.g. psk2, sae, none) */
  readonly encryption: string;
  readonly aps: readonly WifiScanResult[];
}

/** Type guard for WifiScanResult */
export function isWifiScanResult(value: unknown): value is WifiScanResult {
  if (typeof value !== 'object' || value === null) return false;
  const v = value as Record<string, unknown>;
  return (
    typeof v.ssid === 'string' &&
    typeof v.bssid === 'string' &&
    typeof v.channel === 'number' &&
    typeof v.signal_dbm === 'number' &&
    typeof v.signal_percent === 'number' &&
    typeof v.encryption === 'string' &&
    typeof v.band === 'string'
  );
}

/** Radio role: which modes are active on this radio hardware */
export type RadioRole = 'ap' | 'sta' | 'both' | 'none';

/** Response from GET /wifi/band-switching */
export interface BandSwitchResponse {
  readonly config: BandSwitchConfig;
  readonly status: BandSwitchStatus;
}

/** Request to reorder saved network priorities */
export interface NetworkPriorityRequest {
  readonly ssids: string[];
}

/** Auto-reconnect configuration */
export interface AutoReconnectConfig {
  readonly enabled: boolean;
}

/** Pending rollback apply state for a WiFi mutation */
export interface WifiApplyState {
  readonly pending: boolean;
  readonly token?: string;
  readonly rollback_timeout_seconds?: number;
  /**
   * Wall-clock seconds ONE confirm call can block on the device while it waits
   * for the newly enabled interfaces to come up. A client that re-POSTs confirm
   * until the rollback deadline must leave this much room, or a probe it starts
   * late is answered only after rpcd has already rolled back. Absent on
   * backends that predate the field; the client then assumes a default.
   */
  readonly probe_budget_seconds?: number;
}

/** Runtime health check — compares iwinfo SSID with netifd wwan device binding */
export interface WifiHealthSTA {
  readonly ifname: string;
  readonly ssid: string;
  readonly associated: boolean;
}

export interface WifiHealthWwan {
  readonly device: string;
  readonly up: boolean;
  readonly ip_address: string;
}
