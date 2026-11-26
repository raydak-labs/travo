import { describe, expect, it } from 'vitest';
import type { WiFiProfileSummary } from './api/gen';
import { moveProfile, savedChangeError, saveError } from './wifi-profiles';

const profiles: WiFiProfileSummary[] = [
  { id: 'a', ssid: 'HomeUplink', security: 'wpa2' },
  { id: 'b', ssid: 'Neighbor2', security: 'wpa2' },
  { id: 'c', ssid: 'Cafe', security: 'open' },
];

describe('moveProfile', () => {
  it('swaps one neighbour and returns every ID', () => {
    expect(moveProfile(profiles, 1, -1)).toEqual(['b', 'a', 'c']);
    expect(moveProfile(profiles, 1, 1)).toEqual(['a', 'c', 'b']);
  });

  it('refuses to move past either end', () => {
    expect(moveProfile(profiles, 0, -1)).toBeUndefined();
    expect(moveProfile(profiles, 2, 1)).toBeUndefined();
  });
});

describe('saved uplink errors', () => {
  it('names the cause', () => {
    expect(saveError('wifi_profile_exists')).toBe('This uplink is already saved.');
    expect(saveError('wifi_profile_limit')).toBe('You can save up to 32 uplinks.');
    expect(saveError(undefined)).toBe('Could not save the uplink.');
    expect(savedChangeError('wifi_profile_not_found')).toBe('This saved uplink no longer exists.');
    expect(savedChangeError('wifi_profiles_unavailable')).toBe('Could not update saved uplinks.');
  });
});
