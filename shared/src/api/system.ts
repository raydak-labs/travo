import type { SpeedTestResult, SystemInfo, SystemStats } from '../generated/schemas';

// Generated from the backend's OpenAPI document (make generate).
export type {
  AddSSHKeyRequest,
  Alert,
  AlertThresholds,
  AlertsResponse,
  ButtonActionsRequest,
  CpuStats,
  HardwareButton,
  LEDInfo,
  LEDSchedule,
  LEDStatus,
  LogEntry,
  LogResponse,
  MemoryStats,
  NTPConfig,
  NetworkInterfaceStats,
  SSHKey,
  SSHKeysResponse,
  SetHostnameRequest,
  SetLEDRequest,
  SetupStatus,
  SpeedTestResult,
  StorageStats,
  SystemInfo,
  SystemStats,
  TimezoneConfig,
} from '../generated/schemas';

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

/** Firmware upgrade request options */
export interface FirmwareUpgradeRequest {
  readonly keep_settings: boolean;
}

/** Valid hardware button actions */
export type ButtonAction = 'none' | 'vpn_toggle' | 'wifi_toggle' | 'led_toggle' | 'reboot';

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
