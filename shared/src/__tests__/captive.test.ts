import { describe, it, expect } from 'vitest';
import {
  isCaptivePortalStatus,
  isCaptiveAutoAcceptResult,
  type CaptivePortalStatus,
  type CaptiveAutoAcceptResult,
} from '../api/captive';

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

describe('CaptiveAutoAcceptResult', () => {
  it('validates correct payload', () => {
    const res: CaptiveAutoAcceptResult = {
      ok: true,
      message: 'done',
      detected: false,
      can_reach_internet: true,
      attempts: 1,
    };
    expect(isCaptiveAutoAcceptResult(res)).toBe(true);
  });

  it('rejects invalid data', () => {
    expect(isCaptiveAutoAcceptResult(null)).toBe(false);
    expect(isCaptiveAutoAcceptResult({})).toBe(false);
    expect(isCaptiveAutoAcceptResult({ ok: true })).toBe(false);
  });
});
