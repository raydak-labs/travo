import { describe, expect, it } from 'vitest';
import { networkPathnameToTab, networkTabToPath } from '@/pages/network/network-path-utils';

describe('networkPathnameToTab', () => {
  it('maps configuration and advanced URLs', () => {
    expect(networkPathnameToTab('/network/configuration')).toBe('configuration');
    expect(networkPathnameToTab('/network/advanced')).toBe('advanced');
  });

  it('maps base network URL to status', () => {
    expect(networkPathnameToTab('/network')).toBe('status');
    expect(networkPathnameToTab('/network/')).toBe('status');
  });
});

describe('networkTabToPath', () => {
  it('round-trips with pathnameToTab for known tabs', () => {
    const tabs = ['status', 'configuration', 'advanced'] as const;
    for (const tab of tabs) {
      expect(networkPathnameToTab(networkTabToPath(tab))).toBe(tab);
    }
  });
});
