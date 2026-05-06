/** System identification info */
export interface SystemInfo {
  readonly hostname: string;
  readonly model: string;
  readonly firmware_version: string;
  readonly kernel_version: string;
  readonly uptime_seconds: number;
}

/** Set hostname request payload */
export interface SetHostnameRequest {
  readonly hostname: string;
}

/** Individual LED info */
export interface LEDInfo {
  readonly name: string;
  readonly brightness: number;
}

/** LED status */
export interface LEDStatus {
  readonly stealth_mode: boolean;
  readonly led_count: number;
  readonly leds: LEDInfo[];
}

/** Set LED stealth mode request */
export interface SetLEDRequest {
  readonly stealth_mode: boolean;
}

/** LED schedule configuration */
export interface LEDSchedule {
  readonly enabled: boolean;
  readonly on_time: string;
  readonly off_time: string;
}

/** CPU statistics */
export interface CpuStats {
  readonly usage_percent: number;
  readonly cores: number;
  readonly temperature_celsius?: number;
  readonly load_average: readonly [number, number, number];
}

/** Memory statistics */
export interface MemoryStats {
  readonly total_bytes: number;
  readonly used_bytes: number;
  readonly free_bytes: number;
  readonly cached_bytes: number;
  readonly usage_percent: number;
}

/** Storage statistics */
export interface StorageStats {
  readonly total_bytes: number;
  readonly used_bytes: number;
  readonly free_bytes: number;
  readonly usage_percent: number;
}

/** Network interface traffic counters */
export interface NetworkInterfaceStats {
  readonly interface: string;
  readonly rx_bytes: number;
  readonly tx_bytes: number;
}

/** Aggregated system statistics */
export interface SystemStats {
  readonly cpu: CpuStats;
  readonly memory: MemoryStats;
  readonly storage: StorageStats;
  readonly network: readonly NetworkInterfaceStats[];
}

/** Type guard for SystemInfo */
export function isSystemInfo(value: unknown): value is SystemInfo {
  if (typeof value !== 'object' || value === null) return false;
  const v = value as Record<string, unknown>;
  return (
    typeof v.hostname === 'string' &&
    typeof v.model === 'string' &&
    typeof v.firmware_version === 'string' &&
    typeof v.kernel_version === 'string' &&
    typeof v.uptime_seconds === 'number'
  );
}

/** Type guard for SystemStats */
export function isSystemStats(value: unknown): value is SystemStats {
  if (typeof value !== 'object' || value === null) return false;
  const v = value as Record<string, unknown>;
  return (
    typeof v.cpu === 'object' &&
    v.cpu !== null &&
    typeof v.memory === 'object' &&
    v.memory !== null &&
    typeof v.storage === 'object' &&
    v.storage !== null
  );
}

/** Syslog severity levels ordered from most to least severe */
export const LOG_LEVELS = [
  'emerg',
  'alert',
  'crit',
  'err',
  'warning',
  'notice',
  'info',
  'debug',
] as const;
export type LogLevel = (typeof LOG_LEVELS)[number];

/** A single log entry */
export interface LogEntry {
  readonly line: string;
  readonly level: string;
}

/** Response from log retrieval endpoints */
export interface LogResponse {
  readonly source: string;
  readonly lines: readonly LogEntry[];
  readonly total: number;
}

/** Timezone configuration */
export interface TimezoneConfig {
  readonly zonename: string;
  readonly timezone: string;
}

/** NTP time synchronization configuration */
export interface NTPConfig {
  readonly enabled: boolean;
  readonly servers: readonly string[];
}

/** Firmware upgrade request options */
export interface FirmwareUpgradeRequest {
  readonly keep_settings: boolean;
}

/** Setup completion status */
export interface SetupStatus {
  readonly complete: boolean;
}

/** System alert notification */
export interface Alert {
  readonly id: string;
  readonly type: string;
  readonly message: string;
  readonly severity: 'info' | 'warning' | 'critical';
  readonly timestamp: number;
}

/** Response from GET /api/v1/system/alerts */
export interface AlertsResponse {
  readonly alerts: readonly Alert[];
}

/** Valid hardware button actions */
export type ButtonAction = 'none' | 'vpn_toggle' | 'wifi_toggle' | 'led_toggle' | 'reboot';

/** A hardware button with its configured action */
export interface HardwareButton {
  readonly name: string;
  readonly action: ButtonAction;
}

/** Payload for PUT /api/v1/system/button-actions */
export interface ButtonActionsRequest {
  readonly buttons: readonly HardwareButton[];
}

/** A single authorized SSH public key */
export interface SSHKey {
  readonly index: number;
  readonly comment: string;
  readonly key: string;
}

/** Response from GET /api/v1/system/ssh-keys */
export interface SSHKeysResponse {
  readonly keys: readonly SSHKey[];
}

/** Request to add a new SSH key */
export interface AddSSHKeyRequest {
  readonly key: string;
}

/** Result from POST /api/v1/system/speed-test */
export interface SpeedTestResult {
  readonly download_mbps: number;
  readonly upload_mbps: number;
  readonly ping_ms: number;
  readonly server: string;
}

export function isSpeedTestResult(value: unknown): value is SpeedTestResult {
  if (typeof value !== 'object' || value === null) return false;
  const v = value as Record<string, unknown>;
  return (
    typeof v.download_mbps === 'number' &&
    typeof v.upload_mbps === 'number' &&
    typeof v.ping_ms === 'number' &&
    typeof v.server === 'string'
  );
}

/** Configurable alert thresholds */
export interface AlertThresholds {
  readonly storage_percent: number;
  readonly cpu_percent: number;
  readonly memory_percent: number;
}
