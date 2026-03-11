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
  WireGuardStatus,
  WanConfig,
  Client,
  DHCPConfig,
  DNSConfig,
  LogResponse,
  TimezoneConfig,
  APConfig,
  MACConfig,
  DHCPLease,
  GuestWifiConfig,
  DNSEntry,
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
      alias: "John's Laptop",
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
      alias: 'Living Room iPad',
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

export const mockWireguardStatus: WireGuardStatus = {
  interface: 'wg0',
  public_key: 'cHVibGlja2V5aWZhY2U=',
  listen_port: 51820,
  peers: [
    {
      public_key: 'cGVlcnB1YmxpY2tleTE=',
      endpoint: '1.2.3.4:51820',
      latest_handshake: Math.floor(Date.now() / 1000) - 92,
      transfer_rx: 129536789,
      transfer_tx: 71234567,
      allowed_ips: '0.0.0.0/0',
    },
  ],
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
    alias: "John's Laptop",
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
    alias: 'Living Room iPad',
    interface_name: 'br-lan',
    rx_bytes: 209715200,
    tx_bytes: 104857600,
    connected_since: '2026-03-04T10:00:00Z',
  },
];

export const mockDHCPConfig: DHCPConfig = {
  start: 100,
  limit: 150,
  lease_time: '12h',
};

export const mockDNSConfig: DNSConfig = {
  use_custom_dns: false,
  servers: [],
};

export const mockSystemLogs: LogResponse = {
  source: 'system',
  lines: [
    { line: 'Tue Mar 11 09:17:50 2026 daemon.info dnsmasq[1234]: query[A] google.com from 192.168.8.100' },
    { line: 'Tue Mar 11 09:17:51 2026 daemon.info AdGuardHome[3732]: blocked ad.tracker.com' },
    { line: 'Tue Mar 11 09:17:52 2026 daemon.info dnsmasq[1234]: forwarded google.com to 8.8.8.8' },
    { line: 'Tue Mar 11 09:17:53 2026 kern.info netifd[456]: Interface wan up' },
    { line: 'Tue Mar 11 09:17:54 2026 daemon.info hostapd[789]: wlan0: STA aa:bb:cc:dd:ee:ff associated' },
    { line: 'Tue Mar 11 09:17:55 2026 authpriv.info dropbear[1024]: Password auth succeeded for root' },
    { line: 'Tue Mar 11 09:17:56 2026 daemon.info wireguard: wg0 peer endpoint 10.0.0.1:51820 handshake' },
  ],
  total: 7,
};

export const mockKernelLogs: LogResponse = {
  source: 'kernel',
  lines: [{ line: '2026-03-04T12:00:00Z kern.info Kernel initialized' }],
  total: 1,
};

export const mockTimezoneConfig: TimezoneConfig = {
  zonename: 'UTC',
  timezone: 'UTC0',
};

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
    ssid: 'OpenWrt-Travel-5G',
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
  },
];

export const mockDHCPLeases: DHCPLease[] = [
  {
    expiry: Math.floor(Date.now() / 1000) + 3600,
    mac: 'aa:bb:cc:dd:ee:ff',
    ip: '192.168.8.100',
    hostname: 'laptop-1',
  },
  {
    expiry: Math.floor(Date.now() / 1000) + 7200,
    mac: '11:22:33:44:55:66',
    ip: '192.168.8.101',
    hostname: 'phone-2',
  },
];

export const mockGuestWifi: GuestWifiConfig = {
  enabled: false,
  ssid: 'Guest-Travel',
  encryption: 'psk2',
  key: 'guestpass123',
};

export const mockDNSEntries: DNSEntry[] = [
  {
    name: 'nas',
    ip: '192.168.8.10',
    section: 'dns_nas',
  },
  {
    name: 'printer',
    ip: '192.168.8.20',
    section: 'dns_printer',
  },
];
