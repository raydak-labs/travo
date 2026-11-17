import { describe, expect, it } from 'vitest';
import type { Network, ScanResult, WiFiProfileSummary } from './api/gen';
import { suggestUplink } from './wifi-reconnect';

function network(ssid: string, bssid: string): Network {
  return {
    ssid,
    bands: ['5g'],
    best_signal_dbm: -40,
    security: 'wpa2',
    access_points: [
      {
        bssid,
        band: '5g',
        channel: 36,
        frequency_mhz: 5180,
        signal_dbm: -40,
        quality: 60,
        quality_max: 70,
        security: 'wpa2',
      },
    ],
  };
}

const scan: ScanResult = {
  networks: [network('Cafe', '60:22:32:2f:05:2b'), network('HomeUplink', '94:83:c4:1f:28:3a')],
  hidden: [],
  radios: [],
};

const profiles: WiFiProfileSummary[] = [
  { id: 'a', ssid: 'Hotel', security: 'wpa2' },
  { id: 'b', ssid: 'HomeUplink', security: 'wpa2' },
  { id: 'c', ssid: 'Cafe', security: 'open' },
];

describe('suggestUplink', () => {
  it('picks the highest-priority saved uplink in range', () => {
    expect(suggestUplink(profiles, scan)?.id).toBe('b');
  });

  it('needs a scan', () => {
    expect(suggestUplink(profiles, undefined)).toBeUndefined();
  });

  it('honours a pinned BSSID', () => {
    const pinned: WiFiProfileSummary = {
      id: 'p',
      ssid: 'HomeUplink',
      bssid: '00:11:22:33:44:55',
      security: 'wpa2',
    };
    expect(suggestUplink([pinned], scan)).toBeUndefined();
    expect(suggestUplink([{ ...pinned, bssid: '94:83:C4:1F:28:3A' }], scan)?.id).toBe('p');
  });

  it('is empty when no saved uplink is in range', () => {
    expect(suggestUplink([{ id: 'x', ssid: 'Elsewhere', security: 'open' }], scan)).toBeUndefined();
  });
});
