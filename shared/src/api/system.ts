import type { HardwareButton, SpeedTestResult } from '../generated/model';

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
  PackageUpgrade,
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
  UpdateStatus,
} from '../generated/model';

/** Valid hardware button actions */
export type ButtonAction = HardwareButton['action'];

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
