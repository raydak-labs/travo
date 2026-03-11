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

/** LED status */
export interface LEDStatus {
  readonly stealth_mode: boolean;
  readonly led_count: number;
}

/** Set LED stealth mode request */
export interface SetLEDRequest {
  readonly stealth_mode: boolean;
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
