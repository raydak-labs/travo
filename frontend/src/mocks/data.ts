import type {
  SystemInfo,
  SystemStats,
  NetworkStatus,
  WifiConnection,
  WifiScanResult,
  SavedNetwork,
  VpnStatus,
  ServiceInfo,
  CaptivePortalStatus,
  WireguardConfig,
  TailscaleStatus,
  WanConfig,
  Client,
} from '@shared/index';

export const mockSystemInfo: SystemInfo = {
  hostname: 'GL-MT3000',
  model: 'GL.iNet GL-MT3000 (Beryl AX)',
  firmware_version: 'OpenWrt 23.05.3',
  kernel_version: '5.15.150',
  uptime_seconds: 86432,
};

export const mockSystemStats: SystemStats = {
  cpu: {
    usage_percent: 12.5,
    cores: 2,
    temperature_celsius: 52,
    load_average: [0.15, 0.22, 0.18],
  },
  memory: {
    total_bytes: 536870912,
    used_bytes: 214748365,
    free_bytes: 268435456,
    cached_bytes: 53687091,
    usage_percent: 40,
  },
  storage: {
    total_bytes: 7516192768,
    used_bytes: 1503238554,
    free_bytes: 6012954214,
    usage_percent: 20,
  },
};

export const mockNetworkStatus: NetworkStatus = {
  wan: {
    name: 'wlan-sta0',
    type: 'wifi',
    ip_address: '192.168.1.105',
    netmask: '255.255.255.0',
    gateway: '192.168.1.1',
    dns_servers: ['8.8.8.8', '8.8.4.4'],
    mac_address: '00:1A:2B:3C:4D:5E',
    is_up: true,
    rx_bytes: 1073741824,
    tx_bytes: 536870912,
  },
  lan: {
    name: 'br-lan',
    type: 'lan',
    ip_address: '192.168.8.1',
    netmask: '255.255.255.0',
    gateway: '',
    dns_servers: [],
    mac_address: '00:1A:2B:3C:4D:5F',
    is_up: true,
    rx_bytes: 2147483648,
    tx_bytes: 1073741824,
  },
  interfaces: [
    {
      name: 'wlan-sta0',
      type: 'wifi',
      ip_address: '192.168.1.105',
      netmask: '255.255.255.0',
      gateway: '192.168.1.1',
      dns_servers: ['8.8.8.8', '8.8.4.4'],
      mac_address: '00:1A:2B:3C:4D:5E',
      is_up: true,
      rx_bytes: 1073741824,
      tx_bytes: 536870912,
    },
    {
      name: 'br-lan',
      type: 'lan',
      ip_address: '192.168.8.1',
      netmask: '255.255.255.0',
      gateway: '',
      dns_servers: [],
      mac_address: '00:1A:2B:3C:4D:5F',
      is_up: true,
      rx_bytes: 2147483648,
      tx_bytes: 1073741824,
    },
  ],
  clients: [
    {
      ip_address: '192.168.8.100',
      mac_address: 'AA:BB:CC:DD:EE:01',
      hostname: 'MacBook-Pro',
      interface_name: 'br-lan',
      rx_bytes: 524288000,
      tx_bytes: 262144000,
      connected_since: '2026-03-04T08:00:00Z',
    },
    {
      ip_address: '192.168.8.101',
      mac_address: 'AA:BB:CC:DD:EE:02',
      hostname: 'iPhone-15',
      interface_name: 'br-lan',
      rx_bytes: 104857600,
      tx_bytes: 52428800,
      connected_since: '2026-03-04T09:30:00Z',
    },
    {
      ip_address: '192.168.8.102',
      mac_address: 'AA:BB:CC:DD:EE:03',
      hostname: 'iPad-Air',
      interface_name: 'br-lan',
      rx_bytes: 209715200,
      tx_bytes: 104857600,
      connected_since: '2026-03-04T10:00:00Z',
    },
  ],
  internet_reachable: true,
};

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

export const mockVpnStatus: VpnStatus = {
  type: 'wireguard',
  enabled: true,
  connected: true,
  connected_since: '2026-03-04T06:00:00Z',
  endpoint: 'vpn.example.com:51820',
  rx_bytes: 104857600,
  tx_bytes: 52428800,
};

export const mockServices: ServiceInfo[] = [
  {
    id: 'tailscale',
    name: 'Tailscale',
    description: 'Zero config VPN mesh network',
    state: 'running',
    version: '1.62.0',
    auto_start: true,
  },
  {
    id: 'adguardhome',
    name: 'AdGuard Home',
    description: 'Network-wide ad and tracker blocker',
    state: 'installed',
    version: '0.107.45',
    auto_start: false,
  },
  {
    id: 'wireguard',
    name: 'WireGuard',
    description: 'Fast, modern, secure VPN tunnel',
    state: 'running',
    version: '1.0.20210914',
    auto_start: true,
  },
];

export const mockSavedNetworks: SavedNetwork[] = [
  {
    ssid: 'Hotel_Guest_5G',
    encryption: 'wpa2',
    mode: 'client',
    auto_connect: true,
    priority: 1,
  },
  {
    ssid: 'Home_WiFi',
    encryption: 'wpa3',
    mode: 'client',
    auto_connect: true,
    priority: 2,
  },
  {
    ssid: 'Office_Network',
    encryption: 'wpa2/wpa3',
    mode: 'client',
    auto_connect: false,
    priority: 3,
  },
];

export const mockCaptivePortalStatus: CaptivePortalStatus = {
  detected: false,
  can_reach_internet: true,
};

export const mockCaptivePortalDetected: CaptivePortalStatus = {
  detected: true,
  portal_url: 'http://captive.hotel.com/login',
  can_reach_internet: false,
};

export const mockWireguardConfig: WireguardConfig = {
  private_key: 'cGVlcnByaXZhdGVrZXk=',
  address: '10.0.0.2/32',
  dns: ['1.1.1.1', '8.8.8.8'],
  peers: [
    {
      public_key: 'cGVlcnB1YmxpY2tleTE=',
      endpoint: 'vpn.example.com:51820',
      allowed_ips: ['0.0.0.0/0'],
      last_handshake: '2026-03-04T11:55:00Z',
    },
    {
      public_key: 'cGVlcnB1YmxpY2tleXR3bw==',
      endpoint: 'vpn2.example.com:51820',
      allowed_ips: ['10.0.0.0/24', '192.168.1.0/24'],
      preshared_key: 'cHJlc2hhcmVka2V5',
      last_handshake: '2026-03-04T10:30:00Z',
    },
  ],
};

export const mockTailscaleStatus: TailscaleStatus = {
  installed: true,
  running: true,
  logged_in: true,
  ip_address: '100.100.1.42',
  hostname: 'gl-mt3000',
  exit_node: 'us-east-1',
  exit_node_active: true,
};

export const mockWanConfig: WanConfig = {
  type: 'dhcp',
  interface_name: 'wlan-sta0',
  ip_address: '192.168.1.105',
  netmask: '255.255.255.0',
  gateway: '192.168.1.1',
  dns_servers: ['8.8.8.8', '8.8.4.4'],
  mtu: 1500,
};

export const mockClients: Client[] = [
  {
    ip_address: '192.168.8.100',
    mac_address: 'AA:BB:CC:DD:EE:01',
    hostname: 'MacBook-Pro',
    interface_name: 'br-lan',
    rx_bytes: 524288000,
    tx_bytes: 262144000,
    connected_since: '2026-03-04T08:00:00Z',
  },
  {
    ip_address: '192.168.8.101',
    mac_address: 'AA:BB:CC:DD:EE:02',
    hostname: 'iPhone-15',
    interface_name: 'br-lan',
    rx_bytes: 104857600,
    tx_bytes: 52428800,
    connected_since: '2026-03-04T09:30:00Z',
  },
  {
    ip_address: '192.168.8.102',
    mac_address: 'AA:BB:CC:DD:EE:03',
    hostname: 'iPad-Air',
    interface_name: 'br-lan',
    rx_bytes: 209715200,
    tx_bytes: 104857600,
    connected_since: '2026-03-04T10:00:00Z',
  },
];
