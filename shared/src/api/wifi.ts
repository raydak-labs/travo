/** WiFi operation mode */
export type WifiMode = 'client' | 'ap' | 'repeater';

/** WiFi encryption type */
export type WifiEncryption = 'none' | 'wep' | 'wpa' | 'wpa2' | 'wpa3' | 'wpa2/wpa3';

/** WiFi frequency band */
export type WifiBand = '2.4ghz' | '5ghz' | '6ghz';

/** Result of a WiFi scan */
export interface WifiScanResult {
  readonly ssid: string;
  readonly bssid: string;
  readonly channel: number;
  readonly signal_dbm: number;
  readonly signal_percent: number;
  readonly encryption: WifiEncryption;
  readonly band: WifiBand;
}

/** Active WiFi connection */
export interface WifiConnection {
  readonly ssid: string;
  readonly bssid: string;
  readonly mode: WifiMode;
  readonly signal_dbm: number;
  readonly signal_percent: number;
  readonly channel: number;
  readonly encryption: WifiEncryption;
  readonly band: WifiBand;
  readonly ip_address: string;
  readonly connected: boolean;
}

/** WiFi configuration for connecting */
export interface WifiConfig {
  readonly ssid: string;
  readonly password: string;
  readonly encryption: WifiEncryption;
  readonly mode: WifiMode;
  readonly band: WifiBand;
  readonly hidden: boolean;
  readonly channel?: number;
}

/** Saved WiFi network */
export interface SavedNetwork {
  readonly ssid: string;
  readonly section: string;
  readonly encryption: WifiEncryption;
  readonly mode: WifiMode;
  readonly auto_connect: boolean;
  readonly priority: number;
}

/** Access Point configuration for a radio */
export interface APConfig {
  readonly radio: string;
  readonly band: string;
  readonly ssid: string;
  readonly encryption: string;
  readonly key: string;
  readonly enabled: boolean;
  readonly channel: number;
  readonly section: string;
}

/** Request body for PUT /wifi/ap/:section. Omit `enabled` to leave UCI disabled unchanged. */
export interface APConfigUpdate {
  readonly ssid: string;
  readonly encryption: string;
  readonly key: string;
  readonly enabled?: boolean;
}

/** Persisted repeater radio policy (/etc/travo/repeater-options.json). */
export interface RepeaterOptions {
  readonly allow_ap_on_sta_radio: boolean;
}

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

/** MAC address configuration for an interface */
export interface MACConfig {
  readonly interface: string;
  readonly current_mac: string;
  readonly custom_mac?: string;
}

/** Request to set MAC address */
export interface SetMACRequest {
  readonly mac: string;
}

/** Radio role: which modes are active on this radio hardware */
export type RadioRole = 'ap' | 'sta' | 'both' | 'none';

/** WiFi radio hardware info */
export interface RadioInfo {
  readonly name: string;
  readonly band: string;
  readonly channel: number;
  readonly htmode: string;
  readonly type: string;
  readonly disabled: boolean;
  /** Active role: ap, sta, both, or none */
  readonly role: RadioRole;
}

/** Automatic band switching configuration */
export interface BandSwitchConfig {
  readonly enabled: boolean;
  readonly preferred_band: string;
  readonly check_interval_sec: number;
  readonly down_switch_threshold_dbm: number;
  readonly down_switch_delay_sec: number;
  readonly up_switch_threshold_dbm: number;
  readonly up_switch_delay_sec: number;
  readonly min_viable_signal_dbm: number;
}

/** Band switching real-time monitoring status */
export interface BandSwitchStatus {
  /** State: inactive | monitoring | weak_signal | cooldown */
  readonly state: string;
  readonly current_band: string;
  readonly signal_dbm: number;
  readonly weak_signal_secs: number;
  readonly cooldown_sec: number;
  readonly last_switch_at?: string;
  readonly last_switch_reason?: string;
}

/** Response from GET /wifi/band-switching */
export interface BandSwitchResponse {
  readonly config: BandSwitchConfig;
  readonly status: BandSwitchStatus;
}

/** Guest WiFi network configuration */
export interface GuestWifiConfig {
  readonly enabled: boolean;
  readonly ssid: string;
  readonly encryption: string;
  readonly key: string;
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
}

/** Common WiFi mutation response */
export interface WifiMutationResponse {
  readonly status: string;
  readonly apply?: WifiApplyState;
}

/** MAC randomize response */
export interface RandomizeMACResponse extends WifiMutationResponse {
  readonly mac: string;
}

/** WiFi on/off schedule (cron-based) */
export interface WiFiSchedule {
  readonly enabled: boolean;
  readonly on_time: string; // HH:MM
  readonly off_time: string; // HH:MM
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

export interface WifiHealth {
  /** "ok" — STA associated with IP, or pure AP mode.
   *  "warning" — STA associated but wwan has no lease yet.
   *  "error" — wwan bound to a different device than the associated STA. */
  readonly status: 'ok' | 'warning' | 'error';
  readonly issues: readonly string[];
  readonly sta?: WifiHealthSTA;
  readonly wwan?: WifiHealthWwan;
}

/** Per-network MAC address policy */
export interface MACPolicy {
  readonly ssid: string;
  readonly mac: string;
}

/** Collection of per-network MAC policies */
export interface MACPolicies {
  readonly policies: readonly MACPolicy[];
}
