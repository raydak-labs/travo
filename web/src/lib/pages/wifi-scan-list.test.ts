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

function view(result: Partial<ScanResult>, join?: (network: Network) => void): string {
  return render(WiFiScanList, {
    props: { result: { networks: [], hidden: [], radios: [], ...result }, join },
  }).body.replace(/<!--[^>]*-->/g, '');
}

const trigger = (body: string) =>
  body.match(/<button[^>]*aria-expanded=[^>]*>[\s\S]*?<\/button>/)?.[0] ?? '';

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

  it('puts Join right of the network row, outside the expand trigger', () => {
    const body = view(
      { networks: [network('HomeUplink', ['5g'], [ap('5g', '60:22:32:2f:05:2b')])] },
      () => {},
    );
    const row = trigger(body);
    expect(row).toContain('HomeUplink');
    expect(row).not.toContain('Join');
    expect(body).toMatch(/<\/button>\s*<button[^>]*aria-label="Join HomeUplink"/);
  });

  it('marks network and hidden rows as expandable with a hidden chevron', () => {
    const body = view({
      networks: [network('HomeUplink', ['5g'], [ap('5g', '60:22:32:2f:05:2b')])],
      hidden: [ap('5g', '66:22:32:2f:05:2b')],
    });
    const chevrons = body.match(/<svg[^>]*lucide-chevron-right[^>]*>/g) ?? [];
    expect(chevrons).toHaveLength(2);
    for (const svg of chevrons) {
      expect(svg).toContain('aria-hidden="true"');
      expect(svg).toContain('group-data-[state=open]:rotate-90');
    }
  });

  it('lists each access point with hidden band, signal and security icons', () => {
    const open: AccessPoint = { ...ap('2g', '32:cd:a7:aa:79:cd', -71), security: 'open' };
    const body = view({
      networks: [network('Cafe', ['2g', '5g'], [ap('5g', '60:22:32:2f:05:2b'), open])],
    });
    const rows = body.match(/<li[^>]*>[\s\S]*?<\/li>/g) ?? [];
    expect(rows).toHaveLength(2);
    for (const row of rows) {
      expect(row).toMatch(/<svg[^>]*lucide-radio[^>]*aria-hidden="true"/);
      expect(row).toMatch(/<svg[^>]*lucide-signal[^>]*aria-hidden="true"/);
    }
    expect(rows[0]).toContain('5 GHz · Ch 36');
    expect(rows[0]).toMatch(/<svg[^>]*lucide-lock\s[^>]*aria-hidden="true"/);
    expect(rows[1]).toMatch(/<svg[^>]*lucide-lock-open[^>]*aria-hidden="true"/);
    expect(rows[1]).toContain('-71 dBm');
  });

  it('keeps SSID and band on line one and signal and security on line two', () => {
    const body = view(
      {
        networks: [
          network('A very long uplink name', ['2g', '5g'], [ap('5g', 'aa:bb:cc:dd:ee:ff', -30)]),
        ],
      },
      () => {},
    );
    const row = trigger(body);
    const ssid = row.indexOf('>A very long uplink name</span>');
    const badge = row.indexOf('data-slot="badge"');
    const metaStart = row.indexOf('data-slot="scan-row-meta"');
    expect(ssid).toBeGreaterThan(-1);
    expect(row).not.toMatch(/class="[^"]*\btruncate\b[^"]*">A very long uplink name/);
    expect(badge).toBeGreaterThan(ssid);
    expect(metaStart).toBeGreaterThan(badge);
    const meta = row.match(/<span data-slot="scan-row-meta"[^>]*>([\s\S]*)<\/span><\/button>$/);
    expect(meta?.[1]).toContain('-30 dBm');
    expect(meta?.[1]).toContain('WPA2');
    expect(meta?.[1]).toMatch(/<svg[^>]*lucide-signal[^>]*aria-hidden="true"/);
    expect(meta?.[1]).toMatch(/<svg[^>]*lucide-lock\s[^>]*aria-hidden="true"/);
  });
});
