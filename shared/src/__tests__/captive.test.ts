import { describe, it, expect } from 'vitest';
import { isCaptivePortalStatus, type CaptivePortalStatus } from '../api/captive';

describe('CaptivePortalStatus', () => {
  it('validates correct status without portal_url', () => {
    const status: CaptivePortalStatus = {
      detected: false,
      can_reach_internet: true,
    };
    expect(isCaptivePortalStatus(status)).toBe(true);
  });

  it('validates correct status with portal_url', () => {
    const status: CaptivePortalStatus = {
      detected: true,
      portal_url: 'http://captive.example.com/login',
      can_reach_internet: false,
    };
    expect(isCaptivePortalStatus(status)).toBe(true);
  });

  it('rejects invalid data', () => {
    expect(isCaptivePortalStatus(null)).toBe(false);
    expect(isCaptivePortalStatus({})).toBe(false);
    expect(isCaptivePortalStatus({ detected: true })).toBe(false);
    expect(isCaptivePortalStatus('string')).toBe(false);
  });

  it('rejects wrong types', () => {
    expect(isCaptivePortalStatus({ detected: 'yes', can_reach_internet: true })).toBe(false);
    expect(isCaptivePortalStatus({ detected: true, can_reach_internet: 'yes' })).toBe(false);
  });
});
