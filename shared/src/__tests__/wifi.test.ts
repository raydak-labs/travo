import { describe, it, expect } from 'vitest';
import {
  isWifiScanResult,
  type WifiMode,
  type WifiEncryption,
  type WifiBand,
  type WifiScanResult,
  type WifiConnection,
  type WifiConfig,
  type SavedNetwork,
} from '../api/wifi';

describe('WifiMode', () => {
  it('has correct values', () => {
    const modes: WifiMode[] = ['client', 'ap', 'repeater'];
    expect(modes).toHaveLength(3);
  });
});

describe('WifiEncryption', () => {
  it('has correct values', () => {
    const encryptions: WifiEncryption[] = ['none', 'wep', 'wpa', 'wpa2', 'wpa3', 'wpa2/wpa3'];
    expect(encryptions).toHaveLength(6);
  });
});

describe('WifiBand', () => {
  it('has correct values', () => {
    const bands: WifiBand[] = ['2.4ghz', '5ghz', '6ghz'];
    expect(bands).toHaveLength(3);
  });
});

describe('WifiScanResult', () => {
  const validScan: WifiScanResult = {
    ssid: 'MyNetwork',
    bssid: 'AA:BB:CC:DD:EE:FF',
    channel: 6,
    signal_dbm: -55,
    signal_percent: 70,
    encryption: 'wpa2',
    band: '2.4ghz',
  };

  it('validates a correct WifiScanResult', () => {
    expect(isWifiScanResult(validScan)).toBe(true);
  });

  it('rejects invalid data', () => {
    expect(isWifiScanResult(null)).toBe(false);
    expect(isWifiScanResult({})).toBe(false);
    expect(isWifiScanResult({ ssid: 'test' })).toBe(false);
    expect(isWifiScanResult({ ...validScan, channel: 'six' })).toBe(false);
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
    };
    expect(saved.auto_connect).toBe(true);
    expect(saved.priority).toBe(1);
  });
});
