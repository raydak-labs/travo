import type {
  BandSwitchingResponse,
  EnabledResponse,
  PriorityRequest,
  WifiConnection,
  WifiScanResult,
} from '../generated/model';

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
} from '../generated/model';

/** WiFi operation mode */
export type WifiMode = WifiConnection['mode'];

/** WiFi frequency band */
export type WifiBand = '2.4ghz' | '5ghz' | '6ghz';

/** Group of scan results with same SSID and encryption (dual-band = one group) */
export interface GroupedScanNetwork {
  readonly ssid: string;
  /** Encryption from scan (e.g. psk2, sae, none) */
  readonly encryption: string;
  readonly aps: readonly WifiScanResult[];
}

/** Generated as BandSwitchingResponse. */
export type BandSwitchResponse = BandSwitchingResponse;

/** Generated as PriorityRequest. */
export type NetworkPriorityRequest = PriorityRequest;

/** Generated as EnabledResponse. */
export type AutoReconnectConfig = EnabledResponse;
