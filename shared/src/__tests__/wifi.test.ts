import { describe, it, expect } from 'vitest';
import {
  type WifiBand,
  type WifiConnection,
  type WifiConfig,
  type SavedNetwork,
  type MACConfig,
  type SetMACRequest,
  type RandomizeMACResponse,
} from '../api/wifi';

describe('WifiBand', () => {
  it('has correct values', () => {
    const bands: WifiBand[] = ['2.4ghz', '5ghz', '6ghz'];
    expect(bands).toHaveLength(3);
  });
});

describe('WifiConnection', () => {
  it('validates structure', () => {
    const conn: WifiConnection = {
      ssid: 'MyNetwork',
      bssid: 'AA:BB:CC:DD:EE:FF',
      mode: 'client',
      signal_dbm: -55,
      signal_percent: 70,
      channel: 6,
      encryption: 'wpa2',
      band: '2.4ghz',
      ip_address: '192.168.1.100',
      connected: true,
    };
    expect(conn.connected).toBe(true);
    expect(conn.mode).toBe('client');
  });
});

describe('WifiConfig', () => {
  it('validates structure with optional fields', () => {
    const config: WifiConfig = {
      ssid: 'MyNetwork',
      password: 'secret123',
      encryption: 'wpa2',
      mode: 'client',
      band: '2.4ghz',
      hidden: false,
    };
    expect(config.channel).toBeUndefined();

    const configWithChannel: WifiConfig = { ...config, channel: 11 };
    expect(configWithChannel.channel).toBe(11);
  });
});

describe('SavedNetwork', () => {
  it('validates structure', () => {
    const saved: SavedNetwork = {
      ssid: 'MyNetwork',
      encryption: 'wpa2',
      mode: 'client',
      auto_connect: true,
      priority: 1,
      section: 'wifinet2',
    };
    expect(saved.auto_connect).toBe(true);
    expect(saved.priority).toBe(1);
  });
});

describe('MACConfig', () => {
  it('validates structure', () => {
    const mac: MACConfig = {
      interface: 'sta',
      current_mac: 'AA:BB:CC:DD:EE:FF',
      custom_mac: '02:11:22:33:44:55',
      is_applied: true,
    };
    expect(mac.interface).toBe('sta');
    expect(mac.current_mac).toBe('AA:BB:CC:DD:EE:FF');
    expect(mac.custom_mac).toBe('02:11:22:33:44:55');
  });

  it('allows optional custom_mac', () => {
    const mac: MACConfig = {
      interface: 'sta',
      current_mac: 'AA:BB:CC:DD:EE:FF',
      is_applied: false,
    };
    expect(mac.custom_mac).toBeUndefined();
  });
});

describe('SetMACRequest', () => {
  it('validates structure', () => {
    const req: SetMACRequest = { mac: '02:11:22:33:44:55' };
    expect(req.mac).toBe('02:11:22:33:44:55');
  });

  it('allows empty mac for reset', () => {
    const req: SetMACRequest = { mac: '' };
    expect(req.mac).toBe('');
  });
});

describe('RandomizeMACResponse', () => {
  it('validates structure', () => {
    const resp: RandomizeMACResponse = { status: 'ok', mac: '02:ab:cd:ef:12:34' };
    expect(resp.status).toBe('ok');
    expect(resp.mac).toBe('02:ab:cd:ef:12:34');
  });
});
