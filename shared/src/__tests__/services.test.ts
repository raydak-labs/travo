import { describe, it, expect } from 'vitest';
import {
  isServiceInfo,
  type ServiceState,
  type ServiceInfo,
  type AdGuardStatus,
} from '../api/services';

describe('ServiceState', () => {
  it('has correct values', () => {
    const states: ServiceState[] = ['not_installed', 'installed', 'running', 'stopped', 'error'];
    expect(states).toHaveLength(5);
  });
});

describe('ServiceInfo', () => {
  const validService: ServiceInfo = {
    id: 'adguard',
    name: 'AdGuard Home',
    description: 'DNS-based ad blocker',
    state: 'running',
    auto_start: true,
  };

  it('validates a correct ServiceInfo', () => {
    expect(isServiceInfo(validService)).toBe(true);
  });

  it('accepts optional version', () => {
    const service: ServiceInfo = { ...validService, version: '0.107.43' };
    expect(isServiceInfo(service)).toBe(true);
  });

  it('rejects invalid data', () => {
    expect(isServiceInfo(null)).toBe(false);
    expect(isServiceInfo({})).toBe(false);
    expect(isServiceInfo({ id: 'test' })).toBe(false);
    expect(isServiceInfo('string')).toBe(false);
  });

  it('rejects wrong types', () => {
    expect(isServiceInfo({ ...validService, auto_start: 'yes' })).toBe(false);
    expect(isServiceInfo({ ...validService, id: 123 })).toBe(false);
  });
});

describe('AdGuardStatus', () => {
  it('validates structure', () => {
    const status: AdGuardStatus = {
      enabled: true,
      total_queries: 50000,
      blocked_queries: 15000,
      block_percentage: 30.0,
      avg_response_ms: 12.5,
    };
    expect(status.block_percentage).toBe(30.0);
  });
});
