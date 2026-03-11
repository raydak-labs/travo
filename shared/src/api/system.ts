/** System identification info */
export interface SystemInfo {
  readonly hostname: string;
  readonly model: string;
  readonly firmware_version: string;
  readonly kernel_version: string;
  readonly uptime_seconds: number;
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

/** Aggregated system statistics */
export interface SystemStats {
  readonly cpu: CpuStats;
  readonly memory: MemoryStats;
  readonly storage: StorageStats;
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

/** A single log entry */
export interface LogEntry {
  readonly line: string;
}

/** Response from log retrieval endpoints */
export interface LogResponse {
  readonly source: string;
  readonly lines: readonly LogEntry[];
  readonly total: number;
}
