/**
 * Mock fixtures for the captive routes.
 */
import type { CaptivePortalStatus } from '@shared/index';

export const mockCaptivePortalStatus: CaptivePortalStatus = {
  detected: false,
  can_reach_internet: true,
  dns_bypassed: false,
  dns_bypass_needed: false,
  sta_connected: true,
};
export const mockCaptivePortalDetected: CaptivePortalStatus = {
  detected: true,
  portal_url: 'http://captive.hotel.com/login',
  can_reach_internet: false,
  dns_bypassed: false,
  dns_bypass_needed: true,
  sta_connected: true,
};
