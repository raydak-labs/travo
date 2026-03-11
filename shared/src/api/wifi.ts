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

/** WiFi radio hardware info */
export interface RadioInfo {
  readonly name: string;
  readonly band: string;
  readonly channel: number;
  readonly htmode: string;
  readonly type: string;
  readonly disabled: boolean;
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
