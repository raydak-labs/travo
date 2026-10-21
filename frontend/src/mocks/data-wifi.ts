/**
 * Mock fixtures for the wifi routes.
 */
import type {
  WifiConnection,
  WifiScanResult,
  SavedNetwork,
  APConfig,
  MACConfig,
  GuestWifiConfig,
  RadioInfo,
} from '@shared/index';

export const mockWifiConnection: WifiConnection = {
  ssid: 'Hotel_Guest_5G',
  bssid: '00:11:22:33:44:55',
  mode: 'client',
  signal_dbm: -42,
  signal_percent: 82,
  channel: 36,
  encryption: 'wpa2',
  band: '5ghz',
  ip_address: '192.168.1.105',
  connected: true,
};
export const mockWifiScanResults: WifiScanResult[] = [
  {
    ssid: 'Hotel_Guest_5G',
    bssid: '00:11:22:33:44:55',
    channel: 36,
    signal_dbm: -42,
    signal_percent: 82,
    encryption: 'wpa2',
    band: '5ghz',
  },
  {
    ssid: 'Hotel_Guest_2G',
    bssid: '00:11:22:33:44:56',
    channel: 6,
    signal_dbm: -55,
    signal_percent: 65,
    encryption: 'wpa2',
    band: '2.4ghz',
  },
  {
    ssid: 'Airport_Free_WiFi',
    bssid: 'AA:BB:CC:11:22:33',
    channel: 1,
    signal_dbm: -70,
    signal_percent: 45,
    encryption: 'none',
    band: '2.4ghz',
  },
  {
    ssid: 'Starbucks_WiFi',
    bssid: 'DD:EE:FF:44:55:66',
    channel: 11,
    signal_dbm: -78,
    signal_percent: 30,
    encryption: 'wpa2/wpa3',
    band: '2.4ghz',
  },
  {
    ssid: 'Neighbor_5G',
    bssid: '11:22:33:44:55:66',
    channel: 149,
    signal_dbm: -85,
    signal_percent: 18,
    encryption: 'wpa3',
    band: '5ghz',
  },
  {
    ssid: 'CafeNet',
    bssid: '77:88:99:AA:BB:CC',
    channel: 44,
    signal_dbm: -62,
    signal_percent: 55,
    encryption: 'wpa2',
    band: '5ghz',
  },
];
export const mockSavedNetworks: SavedNetwork[] = [
  {
    ssid: 'Hotel_Guest_5G',
    section: 'wifinet2',
    encryption: 'wpa2',
    mode: 'client',
    auto_connect: true,
    priority: 1,
  },
  {
    ssid: 'Home_WiFi',
    section: 'wifinet3',
    encryption: 'wpa3',
    mode: 'client',
    auto_connect: true,
    priority: 2,
  },
  {
    ssid: 'Office_Network',
    section: 'wifinet4',
    encryption: 'wpa2/wpa3',
    mode: 'client',
    auto_connect: false,
    priority: 3,
  },
];
export const mockAPConfigs: APConfig[] = [
  {
    radio: 'radio0',
    band: '2g',
    ssid: 'OpenWrt-Travel',
    encryption: 'psk2',
    key: 'travel12345',
    enabled: true,
    channel: 6,
    section: 'default_radio0',
  },
  {
    radio: 'radio1',
    band: '5g',
    ssid: 'OpenWrt-Travel',
    encryption: 'psk2',
    key: 'travel12345',
    enabled: true,
    channel: 36,
    section: 'default_radio1',
  },
];
export const mockMACAddresses: MACConfig[] = [
  {
    interface: 'sta',
    current_mac: '94:83:c4:1f:28:3a',
    custom_mac: '',
    is_applied: false,
  },
];
export const mockGuestWifi: GuestWifiConfig = {
  enabled: false,
  ssid: 'Guest-Travel',
  encryption: 'psk2',
  key: 'guestpass123',
};
export const mockRadios: RadioInfo[] = [
  {
    name: 'radio0',
    band: '2g',
    channel: 6,
    htmode: 'HE20',
    type: 'mac80211',
    disabled: false,
    role: 'sta',
  },
  {
    name: 'radio1',
    band: '5g',
    channel: 36,
    htmode: 'HE80',
    type: 'mac80211',
    disabled: false,
    role: 'ap',
  },
];
