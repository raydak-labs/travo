import { describe, expect, it } from 'vitest';
import { setupWifiSignalTier } from '../setup-wifi-step-utils';

describe('setupWifiSignalTier', () => {
  it('maps dBm to 1–4 tiers', () => {
    expect(setupWifiSignalTier(-45)).toBe(4);
    expect(setupWifiSignalTier(-55)).toBe(3);
    expect(setupWifiSignalTier(-65)).toBe(2);
    expect(setupWifiSignalTier(-80)).toBe(1);
  });
});
