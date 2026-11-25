import { render } from 'svelte/server';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ScanResult, WiFiProfileList, WiFiReconnectStatus } from '../api/gen';
import WiFi from './WiFi.svelte';

const { createLoader, fetchWifi, fetchWifiProfiles, fetchWifiReconnect, fetchWifiScan } =
  vi.hoisted(() => ({
    createLoader: vi.fn(),
    fetchWifi: vi.fn(),
    fetchWifiProfiles: vi.fn(),
    fetchWifiReconnect: vi.fn(),
    fetchWifiScan: vi.fn(),
  }));
vi.mock('../ui/loader.svelte', () => ({ createLoader }));
vi.mock('../api/wifi', () => ({ fetchWifi, fetchWifiProfiles, fetchWifiReconnect, fetchWifiScan }));
// The access point, uplink MAC and schedule cards have their own tests.
vi.mock('./WiFiAccessPoints.svelte', () => ({ default: () => {} }));
vi.mock('./WiFiUplinkMAC.svelte', () => ({ default: () => {} }));
vi.mock('./WiFiScheduleCard.svelte', () => ({ default: () => {} }));

const summary = { radios: [], stas: [], aps: [] };
// HomeUplink and Neighbor2 are the mock device's scan fixtures.
let saved: WiFiProfileList = { profiles: [], connected_id: null };
let reconnect: WiFiReconnectStatus = { state: 'idle', attempts: 0, max_attempts: 5 };
const scan: ScanResult = {
  networks: [
    {
      ssid: 'HomeUplink',
      bands: ['5g'],
      best_signal_dbm: -30,
      security: 'wpa2',
      access_points: [
        {
          bssid: '60:22:32:2f:05:2b',
          band: '5g',
          channel: 149,
          frequency_mhz: 5745,
          signal_dbm: -30,
          quality: 70,
          quality_max: 70,
          security: 'wpa2',
        },
      ],
    },
  ],
  hidden: [],
  radios: [{ radio: 'radio0', band: '5g', status: 'ok', ap_count: 1 }],
};

describe('WiFi page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    saved = { profiles: [], connected_id: null };
    reconnect = { state: 'idle', attempts: 0, max_attempts: 5 };
    // Lazy panels (PageHost) stay loading; summary, saved uplinks and reconnect have data.
    createLoader.mockImplementation((fn) =>
      fn === fetchWifi || fn === fetchWifiProfiles || fn === fetchWifiReconnect
        ? {
            status: { state: 'ready' },
            data:
              fn === fetchWifiProfiles ? saved : fn === fetchWifiReconnect ? reconnect : summary,
            retry: vi.fn(),
            stop: vi.fn(),
          }
        : { status: { state: 'loading' }, data: undefined, retry: vi.fn() },
    );
  });

  it('lists saved uplinks in priority order with ends disabled', () => {
    saved = {
      profiles: [
        { id: 'a'.repeat(32), ssid: 'Neighbor2', security: 'wpa2' },
        { id: 'b'.repeat(32), ssid: 'HomeUplink', bssid: '94:83:c4:1f:28:3a', security: 'wpa2' },
      ],
      connected_id: 'b'.repeat(32),
    };
    const body = render(WiFi).body;
    expect(body).toContain('Saved uplinks');
    expect(body.indexOf('Neighbor2')).toBeLessThan(body.indexOf('HomeUplink'));
    expect(body).toContain('BSSID 94:83:c4:1f:28:3a');
    expect(body).toContain('Connected');
    expect(body).toMatch(
      /disabled[^>]*aria-label="Move Neighbor2 up"|aria-label="Move Neighbor2 up"[^>]*disabled/,
    );
    expect(body).toMatch(
      /disabled[^>]*aria-label="Move HomeUplink down"|aria-label="Move HomeUplink down"[^>]*disabled/,
    );
    expect(body).not.toContain('sentinel');
    expect(createLoader).toHaveBeenCalledWith(fetchWifiProfiles);
  });

  it('shows an empty saved list as empty, not absent', () => {
    expect(render(WiFi).body).toContain('No saved uplinks. Save one from a nearby network.');
  });

  it('starts idle and never scans automatically', () => {
    const body = render(WiFi).body;
    expect(body).toContain('Scan to see nearby networks.');
    expect(body).toContain('Scan');
    expect(fetchWifiScan).not.toHaveBeenCalled();
    expect(createLoader).toHaveBeenCalledWith(fetchWifi);
    expect(body).toContain('Access point settings');
  });

  it('announces a slow scan and disables the button', () => {
    const body = render(WiFi, { props: { initialScanStatus: { state: 'loading' } } }).body;
    expect(body).toContain('Scanning, this can take several seconds');
    expect(body).toContain('aria-busy="true"');
    expect(body).toContain('disabled');
  });

  it('keeps prior scan data dimmed and stale on error with Retry', () => {
    const body = render(WiFi, {
      props: {
        initialScanStatus: {
          state: 'error',
          message: 'No radio could complete the scan. Try again.',
        },
        initialScanData: scan,
      },
    }).body;
    expect(body).toContain('HomeUplink');
    expect(body).toContain('opacity-60');
    expect(body).toContain('Last known value');
    expect(body).toContain('Retry');
  });

  it('shows reconnect attempts while retrying', () => {
    reconnect = { state: 'retrying', attempts: 2, max_attempts: 5 };
    expect(render(WiFi).body).toContain('Uplink lost. Reconnecting, attempt 2 of 5.');
    expect(createLoader).toHaveBeenCalledWith(fetchWifiReconnect, 30_000);
  });

  it('offers the highest-priority saved uplink in range once the cap is reached', () => {
    reconnect = { state: 'exhausted', attempts: 5, max_attempts: 5 };
    saved = {
      profiles: [
        { id: 'a'.repeat(32), ssid: 'Hotel', security: 'wpa2' },
        { id: 'b'.repeat(32), ssid: 'HomeUplink', security: 'wpa2' },
      ],
      connected_id: null,
    };
    const body = render(WiFi, { props: { initialScanData: scan } }).body;
    expect(body).toContain('Automatic reconnect stopped after 5 attempts.');
    expect(body).toContain('HomeUplink is a saved uplink in range.');
    expect(body).not.toContain('Hotel is a saved uplink in range.');
  });

  it('asks for a scan when the cap is reached and nothing was scanned', () => {
    reconnect = { state: 'exhausted', attempts: 5, max_attempts: 5 };
    expect(render(WiFi).body).toContain('Scan to find a saved uplink in range.');
  });

  it('shows no reconnect notice while idle', () => {
    expect(render(WiFi).body).not.toContain('Uplink lost');
  });
});
