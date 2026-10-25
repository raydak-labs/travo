import { describe, expect, it } from 'vitest';
import { savedNetworkStatus } from '../wifi-connect-utils';

const active = { ssid: 'Cappuxinno', auto_connect: true };

describe('savedNetworkStatus', () => {
  it('reports the active profile as connected while the uplink is associated with it', () => {
    expect(savedNetworkStatus(active, { connected: true, ssid: 'Cappuxinno' })).toBe('connected');
  });

  // On the device the enabled profile read "In use" while the page said
  // "Not connected to any WiFi network": the network was out of range.
  it('reports an enabled profile the uplink is not associated with as not connected', () => {
    expect(savedNetworkStatus(active, { connected: false, ssid: '' })).toBe('not-connected');
    expect(savedNetworkStatus(active, undefined)).toBe('not-connected');
  });

  it('reports every other profile as standby', () => {
    expect(
      savedNetworkStatus(
        { ssid: 'KH-WLan', auto_connect: false },
        { connected: true, ssid: 'Cappuxinno' },
      ),
    ).toBe('standby');
  });
});
