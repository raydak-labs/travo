import { describe, it, expect } from 'vitest';
import {
  isSystemInfo,
  isSystemStats,
  type SystemInfo,
  type CpuStats,
  type MemoryStats,
  type StorageStats,
  type SystemStats,
} from '../api/system';

describe('SystemInfo', () => {
  const validSystemInfo: SystemInfo = {
    hostname: 'OpenWrt',
    model: 'GL-MT3000',
    firmware_version: '23.05.2',
    kernel_version: '5.15.137',
    uptime_seconds: 86400,
  };

  it('validates a correct SystemInfo', () => {
    expect(isSystemInfo(validSystemInfo)).toBe(true);
  });

  it('rejects missing fields', () => {
    expect(isSystemInfo({})).toBe(false);
    expect(isSystemInfo({ hostname: 'test' })).toBe(false);
    expect(isSystemInfo(null)).toBe(false);
    expect(isSystemInfo(undefined)).toBe(false);
    expect(isSystemInfo('string')).toBe(false);
  });

  it('rejects wrong field types', () => {
    expect(isSystemInfo({ ...validSystemInfo, uptime_seconds: '100' })).toBe(false);
    expect(isSystemInfo({ ...validSystemInfo, hostname: 123 })).toBe(false);
  });
});

describe('SystemStats', () => {
  const validCpu: CpuStats = {
    usage_percent: 45.2,
    cores: 4,
    load_average: [0.5, 0.8, 1.2],
  };

  const validCpuWithTemp: CpuStats = {
    ...validCpu,
    temperature_celsius: 55.0,
  };

  const validMemory: MemoryStats = {
    total_bytes: 1073741824,
    used_bytes: 536870912,
    free_bytes: 268435456,
    cached_bytes: 268435456,
    usage_percent: 50.0,
  };

  const validStorage: StorageStats = {
    total_bytes: 8589934592,
    used_bytes: 2147483648,
    free_bytes: 6442450944,
    usage_percent: 25.0,
  };

  const validStats: SystemStats = {
    cpu: validCpu,
    memory: validMemory,
    storage: validStorage,
    network: [{ interface: 'br-lan', rx_bytes: 1024, tx_bytes: 512 }],
  };

  it('validates correct SystemStats', () => {
    expect(isSystemStats(validStats)).toBe(true);
  });

  it('accepts optional temperature_celsius', () => {
    const stats: SystemStats = { ...validStats, cpu: validCpuWithTemp };
    expect(isSystemStats(stats)).toBe(true);
  });

  it('rejects invalid SystemStats', () => {
    expect(isSystemStats(null)).toBe(false);
    expect(isSystemStats({})).toBe(false);
    expect(isSystemStats({ cpu: validCpu })).toBe(false);
  });
});
