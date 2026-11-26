import { render } from 'svelte/server';
import { describe, expect, it } from 'vitest';
import type { AccessPoint, Network, ScanResult } from '../api/gen';
import { accessPointKey } from '../wifi';
import WiFiScanList from './WiFiScanList.svelte';

function ap(band: AccessPoint['band'], bssid: string, signal = -50): AccessPoint {
  return {
    band,
    bssid,
    channel: band === '2g' ? 1 : 36,
    frequency_mhz: band === '2g' ? 2412 : 5180,
    signal_dbm: signal,
    quality: 60,
    quality_max: 70,
    security: 'wpa2',
  };
}

function network(ssid: string, bands: Network['bands'], accessPoints: AccessPoint[]): Network {
  return {
    ssid,
    bands,
    best_signal_dbm: accessPoints[0]?.signal_dbm ?? -50,
    security: 'wpa2',
    access_points: accessPoints,
  };
}

function view(result: Partial<ScanResult>): string {
  return render(WiFiScanList, {
    props: { result: { networks: [], hidden: [], radios: [], ...result } },
  }).body;
}

describe('WiFi scan list', () => {
  it('shows dual and single band badges and discloses BSSIDs', () => {
    const body = view({
      networks: [
        network(
          'HomeUplink',
          ['2g', '5g'],
          [ap('5g', '60:22:32:2f:05:2b'), ap('2g', '60:22:32:2f:05:2a')],
        ),
        network('Neighbor3', ['5g'], [ap('5g', 'a4:ce:da:51:f2:cf')]),
      ],
    });
    expect(body).toContain('2.4 + 5 GHz');
    expect(body).toContain('5 GHz');
    expect(body).toMatch(/<button[^>]*aria-expanded="false"/);
    expect(body).toContain('60:22:32:2f:05:2b');
    expect(body).toContain('a4:ce:da:51:f2:cf');
    expect(body).toContain('WPA2');
  });

  it('keys duplicate BSSIDs by band and list index', () => {
    const duplicate = '60:22:32:2f:05:2b';
    const accessPoints = [ap('2g', duplicate), ap('2g', duplicate), ap('5g', duplicate)];
    const keys = accessPoints.map((accessPoint, index) => accessPointKey(accessPoint, index));
    expect(new Set(keys).size).toBe(accessPoints.length);
    expect(() =>
      view({
        networks: [network('Duplicate', ['2g', '5g'], accessPoints)],
        hidden: [ap('5g', duplicate)],
      }),
    ).not.toThrow();
  });

  it('shows the hidden row only when hidden APs exist', () => {
    expect(
      view({ networks: [network('Visible', ['5g'], [ap('5g', 'aa:bb:cc:dd:ee:ff')])] }),
    ).not.toContain('Hidden networks');
    const body = view({ hidden: [ap('5g', '66:22:32:2f:05:2b')] });
    expect(body).toContain('Hidden networks (1)');
    expect(body).toContain('66:22:32:2f:05:2b');
  });

  it('renders a timed-out radio as danger while keeping network rows', () => {
    const body = view({
      networks: [network('StillHere', ['2g'], [ap('2g', '32:cd:a7:aa:79:cd')])],
      radios: [
        { radio: 'radio0', band: '5g', status: 'error', code: 'scan_timeout', ap_count: 0 },
        { radio: 'radio1', band: '2g', status: 'ok', ap_count: 1 },
      ],
    });
    expect(body).toContain('StillHere');
    expect(body).toContain('5 GHz scan timed out');
    expect(body).toContain('status-danger');
  });

  it('renders a skipped radio as neutral', () => {
    const body = view({
      radios: [{ radio: 'radio1', band: '2g', status: 'skipped', ap_count: 0 }],
    });
    expect(body).toContain('2.4 GHz radio disabled');
    expect(body).toContain('status-neutral');
  });

  it('shows no networks only when every radio succeeded', () => {
    expect(
      view({ radios: [{ radio: 'radio0', band: '5g', status: 'ok', ap_count: 0 }] }),
    ).toContain('No networks found');
    expect(
      view({
        radios: [
          { radio: 'radio0', band: '5g', status: 'error', code: 'scan_failed', ap_count: 0 },
        ],
      }),
    ).not.toContain('No networks found');
  });
});
