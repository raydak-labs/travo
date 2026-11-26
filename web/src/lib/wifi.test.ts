import { describe, expect, it } from 'vitest';
import {
  bandLabel,
  bandsBadge,
  securityLabel,
  signalLabel,
  wifiNotSetUp,
  wifiScanErrorMessage,
} from './wifi';

describe('WiFi labels', () => {
  it('labels bands and dual-band badges', () => {
    expect(bandLabel('2g')).toBe('2.4 GHz');
    expect(bandLabel('5g')).toBe('5 GHz');
    expect(bandLabel('6g')).toBe('6 GHz');
    expect(bandLabel('unknown')).toBe('Unknown band');
    expect(bandsBadge(['2g', '5g'])).toBe('2.4 + 5 GHz');
    expect(bandsBadge(['5g'])).toBe('5 GHz');
  });

  it('labels signal and security', () => {
    expect(signalLabel(-55)).toBe('-55 dBm');
    expect(securityLabel('open')).toBe('Open');
    expect(securityLabel('wpa2')).toBe('WPA2');
    expect(securityLabel('wpa2_wpa3')).toBe('WPA2/WPA3');
    expect(securityLabel('other')).toBe('Other');
  });

  it('keeps scan failure codes distinct', () => {
    expect(
      wifiScanErrorMessage(Object.assign(new Error('fallback'), { code: 'wifi_scan_failed' })),
    ).toBe('No radio could complete the scan. Try again.');
    expect(
      wifiScanErrorMessage(Object.assign(new Error('fallback'), { code: 'wifi_unavailable' })),
    ).toBe('WiFi is unavailable. Try again.');
  });
});

const ap = (over: Partial<{ guest: boolean; enabled: boolean; security: string }> = {}) => ({
  guest: false,
  enabled: false,
  security: 'open',
  ...over,
});

describe('wifiNotSetUp', () => {
  it('is true for the factory APs: off and open', () => {
    expect(wifiNotSetUp([ap(), ap()])).toBe(true);
  });
  it('is false once one main AP is on or has a key', () => {
    expect(wifiNotSetUp([ap(), ap({ enabled: true })])).toBe(false);
    expect(wifiNotSetUp([ap({ security: 'protected' })])).toBe(false);
  });
  it('ignores guest APs and needs at least one main AP', () => {
    expect(wifiNotSetUp([ap({ guest: true })])).toBe(false);
    expect(wifiNotSetUp([])).toBe(false);
  });
});
